[CmdletBinding()]
param(
    [ValidateSet("Fast", "Full")]
    [string]$Mode = "Full",
    [switch]$Format,
    [string]$ArtifactDirectory = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).ProviderPath
if ([string]::IsNullOrWhiteSpace($ArtifactDirectory)) {
    $ArtifactDirectory = Join-Path ([IO.Path]::GetTempPath()) ("dev-control-room-verify-" + [guid]::NewGuid().ToString("N"))
}
$artifactPath = if ([IO.Path]::IsPathRooted($ArtifactDirectory)) {
    $ArtifactDirectory
}
else {
    Join-Path $repositoryRoot $ArtifactDirectory
}
$ArtifactDirectory = (New-Item -ItemType Directory -Path $artifactPath -Force).FullName
$logPath = Join-Path $ArtifactDirectory "verification.log"
$summaryPath = Join-Path $ArtifactDirectory "verification-summary.json"
Set-Content -LiteralPath $logPath -Value "Dev Control Room verification ($Mode)" -Encoding utf8
$script:verificationSteps = [System.Collections.Generic.List[object]]::new()
$script:verificationArtifacts = [System.Collections.Generic.List[object]]::new()
$script:toolVersions = [ordered]@{}
$startedAt = Get-Date

function Write-LogLine {
    param([string]$Message)

    Add-Content -LiteralPath $logPath -Value $Message -Encoding utf8
    Write-Host $Message
}

function Assert-Command {
    param([string]$Name)

    if ($null -eq (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "required command not found: $Name"
    }
}

function Invoke-Checked {
    param(
        [string]$Name,
        [string]$FilePath,
        [string[]]$Arguments = @()
    )

    $commandText = ((@($FilePath) + @($Arguments)) -join " ")
    Write-LogLine (">>> " + $commandText)
    $stepStartedAt = Get-Date
    $output = @(& $FilePath @Arguments 2>&1)
    $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
    foreach ($line in $output) {
        Add-Content -LiteralPath $logPath -Value ([string]$line) -Encoding utf8
    }
    $status = if ($exitCode -eq 0) { "PASS" } else { "FAIL" }
    $script:verificationSteps.Add([pscustomobject]@{
        name = $Name
        command = $commandText
        exitCode = $exitCode
        status = $status
        durationSeconds = [math]::Round(((Get-Date) - $stepStartedAt).TotalSeconds, 3)
    })
    if ($exitCode -ne 0) {
        throw "$Name failed with exit code $exitCode"
    }
    return ,$output
}

function Invoke-WithEnvironment {
    param(
        [hashtable]$Values,
        [scriptblock]$Body
    )

    $previous = @{}
    $present = @{}
    try {
        foreach ($name in $Values.Keys) {
            $present[$name] = $null -ne (Get-Item -Path ("Env:" + $name) -ErrorAction SilentlyContinue)
            $previous[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
            Set-Item -Path ("Env:" + $name) -Value ([string]$Values[$name])
        }
        & $Body
    }
    finally {
        foreach ($name in $Values.Keys) {
            if ($present[$name]) {
                Set-Item -Path ("Env:" + $name) -Value $previous[$name]
            }
            else {
                Remove-Item -Path ("Env:" + $name) -ErrorAction SilentlyContinue
            }
        }
    }
}

function Write-Summary {
    param(
        [string]$Status,
        [string]$Failure = ""
    )

    $sourceCommit = "unknown"
    try {
        $sourceCommit = (& git -c ("safe.directory=" + $repositoryRoot) -C $repositoryRoot rev-parse HEAD).Trim()
    }
    catch {
        $sourceCommit = "unavailable"
    }
    $summary = [ordered]@{
        status = $Status
        mode = $Mode
        sourceCommit = $sourceCommit
        repositoryRoot = $repositoryRoot
        startedAt = $startedAt.ToString("o")
        completedAt = (Get-Date).ToString("o")
        powershell = $PSVersionTable.PSVersion.ToString()
        toolVersions = $script:toolVersions
        steps = @($script:verificationSteps)
        artifacts = @($script:verificationArtifacts)
        log = $logPath
    }
    if (-not [string]::IsNullOrWhiteSpace($Failure)) {
        $summary.failure = $Failure
    }
    $summary | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $summaryPath -Encoding utf8
}

$status = "PASS"
$failure = ""
try {
    foreach ($command in @("git", "go", "gofmt", "node")) {
        Assert-Command $command
    }

    $script:toolVersions.git = ((& git --version).Trim())
    $script:toolVersions.go = ((& go version).Trim())
    $script:toolVersions.node = ((& node --version).Trim())
    if ($null -ne (Get-Command gcc -ErrorAction SilentlyContinue)) {
        $script:toolVersions.gcc = ((& gcc --version | Select-Object -First 1).Trim())
    }

    Push-Location $repositoryRoot
    try {
        $goFiles = @(& git -c ("safe.directory=" + $repositoryRoot) -C $repositoryRoot ls-files --cached --others --exclude-standard -- '*.go' |
            ForEach-Object { Join-Path $repositoryRoot $_ })
        if ($goFiles.Count -eq 0) {
            throw "no Go files found"
        }

        $formatOutput = @(Invoke-Checked "gofmt-check" "gofmt" (@("-l") + $goFiles))
        $unformatted = @($formatOutput | ForEach-Object { [string]$_ } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
        if ($unformatted.Count -gt 0) {
            if (-not $Format) {
                throw ("Go files need formatting: " + ($unformatted -join ", "))
            }
            Invoke-Checked "gofmt-write" "gofmt" (@("-w") + $goFiles) | Out-Null
            Invoke-Checked "gofmt-check-after-write" "gofmt" (@("-l") + $goFiles) | Out-Null
        }

        Invoke-Checked "go-test" "go" @("test", "-count=1", "./...") | Out-Null
        Invoke-Checked "ui-syntax" "node" @("--check", (Join-Path $repositoryRoot "internal\app\ui\app.js")) | Out-Null

        if ($Mode -eq "Full") {
            Invoke-WithEnvironment @{ CGO_ENABLED = "1" } {
                Invoke-Checked "go-test-race" "go" @("test", "-count=1", "-race", "./...") | Out-Null
            }
            Invoke-Checked "go-vet" "go" @("vet", "./...") | Out-Null
            Invoke-Checked "go-mod-verify" "go" @("mod", "verify") | Out-Null
            Invoke-Checked "go-build" "go" @("build", "./...") | Out-Null

            $amd64Binary = Join-Path $ArtifactDirectory "dev-control-room-windows-amd64.exe"
            Invoke-WithEnvironment @{ CGO_ENABLED = "0"; GOOS = "windows"; GOARCH = "amd64" } {
                Invoke-Checked "windows-amd64-build" "go" @("build", "-trimpath", "-o", $amd64Binary, ".\cmd\dev-control-room") | Out-Null
            }
            $script:verificationArtifacts.Add([pscustomobject]@{
                path = $amd64Binary
                sha256 = (Get-FileHash -LiteralPath $amd64Binary -Algorithm SHA256).Hash
            })
            $arm64Binary = Join-Path $ArtifactDirectory "dev-control-room-windows-arm64.exe"
            Invoke-WithEnvironment @{ CGO_ENABLED = "0"; GOOS = "windows"; GOARCH = "arm64" } {
                Invoke-Checked "windows-arm64-build" "go" @("build", "-trimpath", "-o", $arm64Binary, ".\cmd\dev-control-room") | Out-Null
            }
            $script:verificationArtifacts.Add([pscustomobject]@{
                path = $arm64Binary
                sha256 = (Get-FileHash -LiteralPath $arm64Binary -Algorithm SHA256).Hash
            })
        }

        Invoke-Checked "git-diff-check" "git" @("-c", ("safe.directory=" + $repositoryRoot), "-C", $repositoryRoot, "diff", "--check") | Out-Null
    }
    finally {
        Pop-Location
    }
}
catch {
    $status = "FAIL"
    $failure = $_.Exception.Message
    Write-LogLine ("FAILED: " + $failure)
}
finally {
    Write-Summary $status $failure
    Write-Host ("Status: " + $status)
    Write-Host ("Log: " + $logPath)
    Write-Host ("Summary: " + $summaryPath)
}

if ($status -ne "PASS") {
    exit 1
}
