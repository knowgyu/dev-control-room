#requires -Version 7.0
[CmdletBinding()]
param(
    [int]$TimeoutSeconds = 120
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

if (-not [Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([Runtime.InteropServices.OSPlatform]::Windows)) {
    Write-Host "Status: NOT_RUN"
    Write-Host "Reason: this acceptance must execute on native Windows"
    exit 2
}

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).ProviderPath
$tempParent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
$runtimeRoot = Join-Path $tempParent ("dev-control-room-native-resilience-" + [guid]::NewGuid().ToString("N"))
$logPath = Join-Path $runtimeRoot "native-resilience.log"
$script:passCount = 0
$script:failure = $null
$script:stepNumber = 0

function Assert-Condition {
    param(
        [bool]$Condition,
        [string]$Message
    )

    if (-not $Condition) {
        throw $Message
    }
    $script:passCount++
    Write-LogLine ("PASS  " + $Message)
}

function Write-LogLine {
    param([string]$Message)

    if (Test-Path -LiteralPath $logPath -PathType Leaf) {
        Add-Content -LiteralPath $logPath -Value $Message -Encoding utf8
    }
    Write-Host $Message
}

function Get-RegularNativeCommand {
    param([string]$Name)

    $command = Get-Command $Name -CommandType Application -ErrorAction Stop | Select-Object -First 1
    if ($null -eq $command -or [string]::IsNullOrWhiteSpace([string]$command.Source)) {
        throw ("required native command was not found: " + $Name)
    }
    $path = (Resolve-Path -LiteralPath ([string]$command.Source) -ErrorAction Stop).ProviderPath
    $item = Get-Item -LiteralPath $path -Force -ErrorAction Stop
    Assert-Condition ($item.PSIsContainer -eq $false) ("native command is a file: " + $Name)
    Assert-Condition (-not ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) ("native command is not a reparse point: " + $Name)
    return $path
}

function Invoke-Native {
    param(
        [string]$Name,
        [string]$FilePath,
        [string[]]$Arguments = @(),
        [int]$Timeout = $TimeoutSeconds
    )

    $script:stepNumber++
    $commandText = ((@($FilePath) + @($Arguments)) -join " ")
    Write-LogLine (">>> " + $commandText)
    $startInfo = [Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $FilePath
    $startInfo.WorkingDirectory = $repositoryRoot
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    if ($null -eq $startInfo.ArgumentList) {
        throw "typed process argument support is unavailable; use PowerShell 7"
    }
    foreach ($argument in @($Arguments)) {
        [void]$startInfo.ArgumentList.Add([string]$argument)
    }

    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $startInfo
    try {
        if (-not $process.Start()) {
            throw ($Name + " could not start")
        }
        $stdoutTask = $process.StandardOutput.ReadToEndAsync()
        $stderrTask = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit($Timeout * 1000)) {
            $process.Kill($true)
            $process.WaitForExit()
            throw ($Name + " exceeded the bounded timeout")
        }
        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        Write-LogLine ("--- " + $Name + " stdout ---")
        foreach ($line in ($stdout -split "`r?`n")) {
            if (-not [string]::IsNullOrWhiteSpace($line)) {
                Write-LogLine $line
            }
        }
        if (-not [string]::IsNullOrWhiteSpace($stderr)) {
            Write-LogLine ("--- " + $Name + " stderr ---")
            foreach ($line in ($stderr -split "`r?`n")) {
                if (-not [string]::IsNullOrWhiteSpace($line)) {
                    Write-LogLine $line
                }
            }
        }
        return [pscustomobject]@{
            ExitCode = [int]$process.ExitCode
            Stdout   = $stdout
            Stderr   = $stderr
        }
    }
    finally {
        $process.Dispose()
    }
}

function Assert-TestPasses {
    param(
        [string]$Output,
        [string]$TestName
    )

    $escaped = [regex]::Escape($TestName)
    Assert-Condition ($Output -match ("--- PASS: " + $escaped + "(?:\s|$)")) ("native test passed: " + $TestName)
}

try {
    New-Item -ItemType Directory -Path $runtimeRoot -Force | Out-Null
    Set-Content -LiteralPath $logPath -Value "Dev Control Room native resilience acceptance" -Encoding utf8
    Assert-Condition ($PSVersionTable.PSVersion.Major -ge 7) "PowerShell 7 is running"
    $goBinary = Get-RegularNativeCommand "go.exe"
    Assert-Condition ((Get-Date).Kind -eq [DateTimeKind]::Local -or (Get-Date).Kind -eq [DateTimeKind]::Unspecified) "native Windows clock is available"
    $goVersion = Invoke-Native "go-version" $goBinary @("version") 30
    Assert-Condition ($goVersion.ExitCode -eq 0 -and $goVersion.Stdout -match "go version") "native Go toolchain is executable"

    $environmentRun = Invoke-Native "process-runner-resilience" $goBinary @(
        "test", "-count=1", "./internal/environment",
        "-run", "TestProcessRunner(ClosedStdinReturnsEOF|TimeoutKillsProcessTree|CancellationKillsProcessTree|TimeoutCancellationAndBoundedStreams)$",
        "-v"
    )
    Assert-Condition ($environmentRun.ExitCode -eq 0) "native ProcessRunner resilience test command passed"
    $environmentOutput = [string]$environmentRun.Stdout + "`n" + [string]$environmentRun.Stderr
    foreach ($testName in @(
        "TestProcessRunnerClosedStdinReturnsEOF",
        "TestProcessRunnerTimeoutKillsProcessTree",
        "TestProcessRunnerCancellationKillsProcessTree",
        "TestProcessRunnerTimeoutCancellationAndBoundedStreams"
    )) {
        Assert-TestPasses $environmentOutput $testName
    }

    $applicationRun = Invoke-Native "restart-boundary-and-retry" $goBinary @(
        "test", "-count=1", "./internal/app",
        "-run", "TestStartupRecoveryMarksActiveInvocationInterruptedWithoutRelaunch|TestRetryInterruptedInvocationCreatesIdempotentChildWithoutPromptPersistence",
        "-v"
    )
    Assert-Condition ($applicationRun.ExitCode -eq 0) "native restart-boundary and retry test command passed"
    $applicationOutput = [string]$applicationRun.Stdout + "`n" + [string]$applicationRun.Stderr
    foreach ($testName in @(
        "TestStartupRecoveryMarksActiveInvocationInterruptedWithoutRelaunch",
        "TestRetryInterruptedInvocationCreatesIdempotentChildWithoutPromptPersistence"
    )) {
        Assert-TestPasses $applicationOutput $testName
    }
    Assert-Condition ($environmentOutput -notmatch "(?im)^FAIL") "native ProcessRunner output contains no failing test"
    Assert-Condition ($applicationOutput -notmatch "(?im)^FAIL") "native application resilience output contains no failing test"

    Write-LogLine ("Status: PASS (" + $script:passCount + " assertions)")
    Write-LogLine ("Log: " + $logPath)
}
catch {
    $script:failure = $_.Exception.Message
    Write-LogLine ("Status: FAIL - " + $script:failure)
    Write-LogLine ("Assertions passed: " + $script:passCount)
}
finally {
    $resolvedRoot = [IO.Path]::GetFullPath($runtimeRoot).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
    $resolvedParent = [IO.Path]::GetFullPath($tempParent).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
    $prefix = $resolvedParent + [IO.Path]::DirectorySeparatorChar
    if ($resolvedRoot.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase) -and $resolvedRoot -ne $resolvedParent -and (Test-Path -LiteralPath $resolvedRoot)) {
        [System.IO.Directory]::Delete($resolvedRoot, $true)
        Write-Host ("CLEANUP  removed isolated temporary root: " + $resolvedRoot)
    }
}

if ($null -ne $script:failure) {
    exit 1
}
