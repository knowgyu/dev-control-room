#requires -Version 7.0
[CmdletBinding()]
param(
    [string]$BinaryPath = "",
    [switch]$KeepTemp
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).ProviderPath
$tempParent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
$runtimeRoot = Join-Path $tempParent ("dev-control-room-real-codex-" + [guid]::NewGuid().ToString("N"))
$appHome = Join-Path $runtimeRoot "app-home"
$fixturePath = Join-Path $runtimeRoot "git-fixture"
$processOutputRoot = Join-Path $runtimeRoot "process-output"
$script:appBinary = ""
$script:gitBinary = ""
$script:passCount = 0
$script:processNumber = 0
$script:prompt = "Inspect the local repository state and return a short structured summary."
$script:failure = $null
$script:cleanupFailure = $null

function Assert-Condition {
    param(
        [bool]$Condition,
        [string]$Message
    )

    if (-not $Condition) {
        throw $Message
    }
    $script:passCount++
    Write-Host ("PASS  " + $Message)
}

function Get-RegularNativePath {
    param([string]$Path)

    $resolved = (Resolve-Path -LiteralPath $Path -ErrorAction Stop).ProviderPath
    $item = Get-Item -LiteralPath $resolved -Force -ErrorAction Stop
    Assert-Condition ($item.PSIsContainer -eq $false) ("native executable is a file: " + [IO.Path]::GetFileName($resolved))
    Assert-Condition (-not ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) ("native executable is not a reparse point: " + [IO.Path]::GetFileName($resolved))
    return $resolved
}

function Get-CommandPath {
    param([string]$Name)

    $command = Get-Command $Name -CommandType Application -ErrorAction Stop | Select-Object -First 1
    if ($null -eq $command -or [string]::IsNullOrWhiteSpace([string]$command.Source)) {
        throw ("required native command was not found: " + $Name)
    }
    return Get-RegularNativePath ([string]$command.Source)
}

function Invoke-Native {
    param(
        [string]$Name,
        [string]$FilePath,
        [string[]]$Arguments = @(),
        [string]$WorkingDirectory = $repositoryRoot,
        [int]$TimeoutSeconds = 120
    )

    $script:processNumber++
    $stdoutPath = Join-Path $processOutputRoot ("{0:D3}-{1}.stdout.txt" -f $script:processNumber, $Name)
    $stderrPath = Join-Path $processOutputRoot ("{0:D3}-{1}.stderr.txt" -f $script:processNumber, $Name)
    $startInfo = [Diagnostics.ProcessStartInfo]::new()
    $startInfo.FileName = $FilePath
    $startInfo.WorkingDirectory = $WorkingDirectory
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
        if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
            $process.Kill($true)
            $process.WaitForExit()
            throw ($Name + " exceeded the bounded timeout")
        }
        $stdout = $stdoutTask.GetAwaiter().GetResult()
        $stderr = $stderrTask.GetAwaiter().GetResult()
        if ($stdout.Length -gt (256 * 1024) -or $stderr.Length -gt (256 * 1024)) {
            throw ($Name + " exceeded the bounded output limit")
        }
        [IO.File]::WriteAllText($stdoutPath, $stdout, [Text.UTF8Encoding]::new($false))
        [IO.File]::WriteAllText($stderrPath, $stderr, [Text.UTF8Encoding]::new($false))
        return [pscustomobject]@{
            Name = $Name
            ExitCode = [int]$process.ExitCode
            Stdout = $stdout
            Stderr = $stderr
            StdoutPath = $stdoutPath
            StderrPath = $stderrPath
        }
    }
    finally {
        $process.Dispose()
    }
}

function Invoke-CheckedNative {
    param(
        [string]$Name,
        [string]$FilePath,
        [string[]]$Arguments = @(),
        [string]$WorkingDirectory = $repositoryRoot,
        [int]$TimeoutSeconds = 120
    )

    $result = Invoke-Native -Name $Name -FilePath $FilePath -Arguments $Arguments -WorkingDirectory $WorkingDirectory -TimeoutSeconds $TimeoutSeconds
    if ($result.ExitCode -ne 0) {
        throw ($Name + " failed with exit code " + $result.ExitCode)
    }
    return $result
}

function Invoke-CliJson {
    param([string[]]$Arguments)

    $cliArguments = @($Arguments) + @("--home", $appHome, "--json")
    $result = Invoke-CheckedNative -Name ("cli-" + ($Arguments[0] -replace "[^a-zA-Z0-9_-]", "-")) `
        -FilePath $script:appBinary -Arguments $cliArguments -WorkingDirectory $repositoryRoot
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($result.Stdout)) ("CLI returned JSON: " + ($Arguments -join " "))
    try {
        $envelope = $result.Stdout | ConvertFrom-Json
    }
    catch {
        throw ("CLI returned invalid JSON: " + ($Arguments -join " "))
    }
    Assert-Condition ([bool]$envelope.ok) ("CLI envelope is OK: " + ($Arguments -join " "))
    return $envelope.data
}

function Write-Utf8File {
    param(
        [string]$Path,
        [string]$Content
    )

    [IO.File]::WriteAllText($Path, $Content, [Text.UTF8Encoding]::new($false))
}

function Initialize-GitFixture {
    New-Item -ItemType Directory -Path $fixturePath -Force | Out-Null
    Invoke-CheckedNative "git-init" $script:gitBinary @("init", "--quiet", $fixturePath) $repositoryRoot 30 | Out-Null
    Invoke-CheckedNative "git-user-name" $script:gitBinary @("-C", $fixturePath, "config", "user.name", "Real Codex Acceptance") $repositoryRoot 30 | Out-Null
    Invoke-CheckedNative "git-user-email" $script:gitBinary @("-C", $fixturePath, "config", "user.email", "real-codex-acceptance@example.invalid") $repositoryRoot 30 | Out-Null
    Invoke-CheckedNative "git-default-branch" $script:gitBinary @("-C", $fixturePath, "branch", "-M", "main") $repositoryRoot 30 | Out-Null
    Write-Utf8File (Join-Path $fixturePath "README.md") "# Real Codex acceptance fixture`n"
    Write-Utf8File (Join-Path $fixturePath "go.mod") "module example.invalid/real-codex-acceptance`ngo 1.23`n"
    Write-Utf8File (Join-Path $fixturePath "main.go") "package main`n`nfunc main() {}`n"
    Invoke-CheckedNative "git-add" $script:gitBinary @("-C", $fixturePath, "add", "--", "README.md", "go.mod", "main.go") $repositoryRoot 30 | Out-Null
    Invoke-CheckedNative "git-commit" $script:gitBinary @("-C", $fixturePath, "commit", "--quiet", "-m", "initial acceptance fixture") $repositoryRoot 30 | Out-Null
    Assert-Condition ((Test-Path -LiteralPath (Join-Path $fixturePath ".git") -PathType Container)) "fresh local Git fixture exists"
}

function Assert-CodexReadiness {
    $providers = @(Invoke-CliJson @("assurance", "provider"))
    $codex = @($providers | Where-Object { [string]$_.provider -eq "codex" })
    Assert-Condition ($codex.Count -eq 1) "Codex Provider status is present"
    $status = $codex[0]
    Assert-Condition ([string]$status.state -eq "ready" -and $status.commandFound -eq $true -and $status.launchTrusted -eq $true -and $status.profileReady -eq $true) "real Codex Provider is ready"
    $resolved = @($status.resolvedCommand | ForEach-Object { [string]$_ })
    Assert-Condition ($resolved.Count -eq 2) "Codex readiness has exactly two typed command elements"
    Assert-Condition ([IO.Path]::GetFileName($resolved[0]) -ieq "node.exe") "Codex executable is node.exe"
    Assert-Condition ($resolved[1].Replace("\", "/").EndsWith("/node_modules/@openai/codex/bin/codex.js", [StringComparison]::OrdinalIgnoreCase)) "Codex script is the verified @openai/codex bin/codex.js"
    foreach ($part in $resolved) {
        $normalized = $part.Replace("\", "/")
        Assert-Condition ($normalized -notmatch "(?i)(^|/)(cmd|cmd\.exe|[^/]+\.cmd|[^/]+\.bat)$") "Codex typed command contains no shell launcher"
        $item = Get-Item -LiteralPath $part -Force -ErrorAction Stop
        Assert-Condition ($item.PSIsContainer -eq $false -and -not ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) ("Codex command element is a regular file: " + [IO.Path]::GetFileName($part))
    }
    return $status
}

function Assert-CleanFixture {
    $status = Invoke-CheckedNative "git-status" $script:gitBinary @("-C", $fixturePath, "status", "--porcelain") $repositoryRoot 30
    Assert-Condition ([string]::IsNullOrWhiteSpace($status.Stdout)) "Codex leaves the fixture Git status clean"
}

function Assert-PromptAbsentFromAppHome {
    $promptBytes = [Text.UTF8Encoding]::new($false).GetBytes($script:prompt)
    $matches = @()
    foreach ($file in @(Get-ChildItem -LiteralPath $appHome -Recurse -Force -File -ErrorAction Stop)) {
        $bytes = [IO.File]::ReadAllBytes($file.FullName)
        if ($bytes.Length -lt $promptBytes.Length) {
            continue
        }
        for ($index = 0; $index -le ($bytes.Length - $promptBytes.Length); $index++) {
            $found = $true
            for ($offset = 0; $offset -lt $promptBytes.Length; $offset++) {
                if ($bytes[$index + $offset] -ne $promptBytes[$offset]) {
                    $found = $false
                    break
                }
            }
            if ($found) {
                $matches += $file.FullName
                break
            }
        }
    }
    Assert-Condition ($matches.Count -eq 0) "prompt is not persisted under app home"
}

function Assert-SafeTempRoot {
    $target = [IO.Path]::GetFullPath($runtimeRoot).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
    $parent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
    $prefix = $parent + [IO.Path]::DirectorySeparatorChar
    $name = [IO.Path]::GetFileName($target)
    Assert-Condition ($target.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase) -and $target -ne $parent) "temporary root is a child of the OS temp directory"
    Assert-Condition ($name -match "^dev-control-room-real-codex-[0-9a-f]{32}$") "temporary root has the expected unique acceptance name"
    if (Test-Path -LiteralPath $target) {
        $item = Get-Item -LiteralPath $target -Force
        Assert-Condition ($item.PSIsContainer -and -not ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) "temporary root is a regular directory"
    }
}

try {
    Assert-SafeTempRoot
    New-Item -ItemType Directory -Path $runtimeRoot -Force | Out-Null
    New-Item -ItemType Directory -Path $appHome -Force | Out-Null
    New-Item -ItemType Directory -Path $processOutputRoot -Force | Out-Null

    $script:gitBinary = Get-CommandPath "git.exe"
    if ([string]::IsNullOrWhiteSpace($BinaryPath)) {
        $goBinary = Get-CommandPath "go.exe"
        $script:appBinary = Join-Path $runtimeRoot "dev-control-room.exe"
        Invoke-CheckedNative "build" $goBinary @("build", "-trimpath", "-o", $script:appBinary, ".\cmd\dev-control-room") $repositoryRoot 180 | Out-Null
        $script:appBinary = Get-RegularNativePath $script:appBinary
        Assert-Condition (Test-Path -LiteralPath $script:appBinary -PathType Leaf) "current CLI binary was built in the isolated temp root"
    }
    else {
        $script:appBinary = Get-RegularNativePath $BinaryPath
        Assert-Condition ([IO.Path]::GetExtension($script:appBinary) -notin @(".cmd", ".bat")) "provided CLI binary is not a shell launcher"
    }

    Initialize-GitFixture
    $project = Invoke-CliJson @("project", "add", "--name", "Real Codex acceptance", "--path", $fixturePath)
    $projectID = [string]$project.metadata.id
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($projectID)) "fixture project registration returns an id"
    $scan = Invoke-CliJson @("project", "scan")
    Assert-Condition ([string]$scan.status -eq "completed") "fixture scan completes"
    $repositories = @(Invoke-CliJson @("project", "repository", "list", $projectID))
    Assert-Condition ($repositories.Count -eq 1) "registered project has one fixture repository"
    $repositoryID = [string]$repositories[0].metadata.id
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($repositoryID)) "fixture repository registration returns an id"
    $worktrees = @(Invoke-CliJson @("project", "worktree", "list", $projectID, $repositoryID))
    $primary = @($worktrees | Where-Object { [string]$_.metadata.id -eq "primary" })
    Assert-Condition ($primary.Count -eq 1) "fixture has a primary observed Worktree"

    [void](Assert-CodexReadiness)
    $session = Invoke-CliJson @("assurance", "session", "create", "--project", $projectID, "--repository", $repositoryID, "--worktree", "primary", "--provider", "codex")
    $sessionID = [string]$session.metadata.id
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($sessionID)) "Codex Assurance session is created"
    $invocation = Invoke-CliJson @("assurance", "invocation", "run", "--session", $sessionID, "--provider", "codex", "--prompt", $script:prompt)
    Assert-Condition ([string]$invocation.spec.state -eq "succeeded") "real Codex invocation succeeds"
    Assert-Condition ($invocation.spec.rawTranscript -eq $false) "invocation retains no raw transcript"
    Assert-Condition (-not [string]::IsNullOrWhiteSpace([string]$invocation.spec.structured.summary)) "invocation has a structured summary"
    Assert-Condition ($null -ne $invocation.spec.structured.findings -and $null -ne $invocation.spec.structured.nextAction) "invocation has fixed structured fields"
    Assert-Condition (@($invocation.spec.artifactIds).Count -ge 1) "invocation retains bounded evidence"
    Assert-CleanFixture
    Assert-PromptAbsentFromAppHome

    Write-Host ("Status: PASS (" + $script:passCount + " assertions)")
    Write-Host ("Binary: " + $script:appBinary)
}
catch {
    $script:failure = $_.Exception.Message
    Write-Host ("Status: FAIL - " + $script:failure)
    Write-Host ("Assertions passed: " + $script:passCount)
}
finally {
    if ($KeepTemp) {
        try {
            Assert-SafeTempRoot
            Write-Host ("Evidence retained: " + $runtimeRoot)
        }
        catch {
            $script:cleanupFailure = $_.Exception.Message
        }
    }
    else {
        try {
            Assert-SafeTempRoot
            if (Test-Path -LiteralPath $runtimeRoot) {
                Remove-Item -LiteralPath $runtimeRoot -Recurse -Force
            }
            Write-Host "Temporary root cleaned"
        }
        catch {
            $script:cleanupFailure = $_.Exception.Message
        }
    }
}

if ($null -ne $script:cleanupFailure) {
    Write-Error ("cleanup failed closed: " + $script:cleanupFailure)
    exit 1
}
if ($null -ne $script:failure) {
    exit 1
}
