#requires -Version 7.6

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).ProviderPath
$validatorPath = Join-Path $repositoryRoot "scripts\verify-measurement-contract.ps1"
$measurePath = Join-Path $repositoryRoot "scripts\measure-dogfood.ps1"
$testRoot = Join-Path ([IO.Path]::GetTempPath()) ("dev-control-room-measurement-contract-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $testRoot -Force | Out-Null

function New-TestManifest {
    return [ordered]@{
        apiVersion = "devroom/measurement/v1"
        kind = "DogfoodMeasurementRun"
        metadata = [ordered]@{ id = "dogfood-contract-test" }
        spec = [ordered]@{
            status = "unknown"
            requiredFailures = @()
            reproducibility = [ordered]@{
                runId = "dogfood-contract-test"
                commit = ("a" * 40)
                head = ("a" * 40)
                dirtyState = "clean"
                os = "windows"
                arch = "amd64"
                toolVersions = [ordered]@{
                    go = "go1.26.7"
                    gofmt = "go1.26.7"
                }
                toolPaths = [ordered]@{
                    gofmt = "C:\Program Files\Go\bin\gofmt.exe"
                }
                configurationDigest = ("sha256:" + ("0" * 64))
                startedAt = "2026-09-01T00:00:00.0000000Z"
                endedAt = "2026-09-01T00:00:01.0000000Z"
            }
            measurements = @(
                [ordered]@{
                    apiVersion = "devroom/measurement/v1"
                    kind = "Measurement"
                    metadata = [ordered]@{ id = "performance-http-health" }
                    spec = [ordered]@{
                        name = "performance.http.health.latency"
                        category = "performance"
                        status = "unknown"
                        provenance = "measured"
                        unit = "milliseconds"
                        sampleCount = 4
                        rawSamples = @(1, 1, 1, 1)
                        requestCount = 5
                        successCount = 4
                        failureCount = 1
                        failureReasons = @("http_status_503")
                        min = 1
                        p50 = 1
                        p95 = 1
                        max = 1
                        commandId = "http.get.health"
                        command = "GET /api/health"
                        required = $false
                    }
                }
            )
        }
    }
}

function Write-TestManifest {
    param(
        [string]$Path,
        [object]$Manifest
    )

    $Manifest | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $Path -Encoding utf8
}

function Invoke-TestValidator {
    param([string]$Path)

    $output = @(& pwsh -NoProfile -File $validatorPath -ManifestPath $Path 2>&1)
    return [pscustomobject]@{
        ExitCode = [int]$LASTEXITCODE
        Output = @($output | ForEach-Object { [string]$_ })
    }
}

function New-DogfoodToolFixture {
    param(
        [string]$Path,
        [string]$RealGoPath,
        [switch]$FailCanonical,
        [switch]$VanishNode
    )

    New-Item -ItemType Directory -Path $Path -Force | Out-Null
    $canonicalCommand = if ($FailCanonical) {
        "  exit /b 1"
    }
    else {
        "  `"$RealGoPath`" %*`r`n  exit /b %ERRORLEVEL%"
    }
    $goBody = @"
@echo off
if /I "%~1"=="run" if /I "%~2"=="./cmd/verify-measurement-contract" (
$canonicalCommand
)
if /I "%~1"=="version" (
  if /I "%~2"=="-m" (
    echo gofmt build go1.26.7
    echo path cmd/gofmt
  ) else (
    echo go1.26.7
  )
  exit /b 0
)
exit /b 0
"@
    [IO.File]::WriteAllText((Join-Path $Path "go.cmd"), $goBody, [Text.Encoding]::ASCII)
    [IO.File]::WriteAllText((Join-Path $Path "gofmt.cmd"), "@echo off`r`nexit /b 0`r`n", [Text.Encoding]::ASCII)
    $gitBody = @'
@echo off
if /I "%~1"=="--version" (
  echo git version 2.0.0
  exit /b 0
)
if /I "%~3"=="rev-parse" (
  echo aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  exit /b 0
)
exit /b 0
'@
    [IO.File]::WriteAllText((Join-Path $Path "git.cmd"), $gitBody, [Text.Encoding]::ASCII)
    $nodeBody = if ($VanishNode) {
@'
@echo off
if /I "%~1"=="--version" (
  del /f /q "%~f0" >nul 2>&1
  exit /b 0
)
exit /b 0
'@
    }
    else {
@'
@echo off
if /I "%~1"=="--version" (
  echo node v20.0.0
  exit /b 0
)
exit /b 0
'@
    }
    [IO.File]::WriteAllText((Join-Path $Path "node.cmd"), $nodeBody, [Text.Encoding]::ASCII)
}

try {
    Describe "measurement contract boundary" {
        It "uses tracked Go files and verifies the actual gofmt provenance" {
            $source = Get-Content -Raw -LiteralPath $measurePath
            $source | Should Match 'ls-files.*--cached.*\*\.go'
            $source | Should Not Match '--others|--exclude-standard'
            $source | Should Match 'version.*-m.*GofmtPath'
            $source | Should Match 'toolPaths\.gofmt'
            $source | Should Match 'ProvenanceTool "gofmt"'
            $source | Should Match 'verify-measurement-contract'
            $source | Should Match 'Assert-CanonicalManifest'
            $source | Should Match '\$temporaryPath'
            $source | Should Match 'Move-Item'
        }

        It "exports a contract-valid unavailable measurement when a resolved executable disappears before launch" {
            $fixturePath = Join-Path $testRoot "vanishing-node"
            $realGoCommand = Get-Command go -CommandType Application -ErrorAction Stop | Select-Object -First 1
            $realGoPath = [string]$realGoCommand.Source
            New-DogfoodToolFixture -Path $fixturePath -RealGoPath $realGoPath -VanishNode
            $outputPath = Join-Path $testRoot "vanishing-node-output"
            $hadPath = $null -ne (Get-Item -Path Env:Path -ErrorAction SilentlyContinue)
            $previousPath = [Environment]::GetEnvironmentVariable("Path", "Process")
            $hadPathExt = $null -ne (Get-Item -Path Env:PATHEXT -ErrorAction SilentlyContinue)
            $previousPathExt = [Environment]::GetEnvironmentVariable("PATHEXT", "Process")
            try {
                $env:Path = $fixturePath + [IO.Path]::PathSeparator + $previousPath
                $env:PATHEXT = ".CMD;.EXE;.BAT;.COM"
                $null = @(& pwsh -NoProfile -File $measurePath -OutputDirectory $outputPath 2>&1)
                $runExitCode = [int]$LASTEXITCODE
            }
            finally {
                if ($hadPath) {
                    $env:Path = $previousPath
                }
                else {
                    Remove-Item -Path Env:Path -ErrorAction SilentlyContinue
                }
                if ($hadPathExt) {
                    $env:PATHEXT = $previousPathExt
                }
                else {
                    Remove-Item -Path Env:PATHEXT -ErrorAction SilentlyContinue
                }
            }

            $runExitCode | Should Be 1
            $manifestPath = Join-Path $outputPath "dogfood-measurement.json"
            (Test-Path -LiteralPath $manifestPath -PathType Leaf) | Should Be $true
            $validation = Invoke-TestValidator -Path $manifestPath
            $validation.ExitCode | Should Be 0
            $manifest = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
            $uiSyntax = @($manifest.spec.measurements | Where-Object { $_.metadata.id -eq "quality-ui-syntax" })
            $uiSyntax.Count | Should Be 1
            $uiSyntax[0].spec.status | Should Be "unknown"
            $uiSyntax[0].spec.provenance | Should Be "unavailable"
            $uiSyntax[0].spec.sampleCount | Should Be 0
            @($uiSyntax[0].spec.rawSamples).Count | Should Be 0
            $null -eq $uiSyntax[0].spec.exitCode | Should Be $true
            $manifest.spec.status | Should Be "fail"
            @($manifest.spec.requiredFailures).Count | Should Be 1
            $manifest.spec.requiredFailures[0] | Should Be "quality-ui-syntax"
            $manifest.spec.reproducibility.toolVersions.node | Should Be "unavailable"
            $manifest.spec.reproducibility.toolPaths.node | Should Match 'node\.cmd$'
        }

        It "does not replace an existing manifest when canonical preflight rejects the new one" {
            $fixturePath = Join-Path $testRoot "canonical-reject"
            $realGoCommand = Get-Command go -CommandType Application -ErrorAction Stop | Select-Object -First 1
            $realGoPath = [string]$realGoCommand.Source
            New-DogfoodToolFixture -Path $fixturePath -RealGoPath $realGoPath -FailCanonical
            $outputPath = Join-Path $testRoot "canonical-reject-output"
            New-Item -ItemType Directory -Path $outputPath -Force | Out-Null
            $manifestPath = Join-Path $outputPath "dogfood-measurement.json"
            Write-TestManifest -Path $manifestPath -Manifest (New-TestManifest)
            $existingManifest = Get-Content -Raw -LiteralPath $manifestPath
            $hadPath = $null -ne (Get-Item -Path Env:Path -ErrorAction SilentlyContinue)
            $previousPath = [Environment]::GetEnvironmentVariable("Path", "Process")
            $hadPathExt = $null -ne (Get-Item -Path Env:PATHEXT -ErrorAction SilentlyContinue)
            $previousPathExt = [Environment]::GetEnvironmentVariable("PATHEXT", "Process")
            try {
                $env:Path = $fixturePath + [IO.Path]::PathSeparator + $previousPath
                $env:PATHEXT = ".CMD;.EXE;.BAT;.COM"
                $null = @(& pwsh -NoProfile -File $measurePath -OutputDirectory $outputPath 2>&1)
                $runExitCode = [int]$LASTEXITCODE
            }
            finally {
                if ($hadPath) {
                    $env:Path = $previousPath
                }
                else {
                    Remove-Item -Path Env:Path -ErrorAction SilentlyContinue
                }
                if ($hadPathExt) {
                    $env:PATHEXT = $previousPathExt
                }
                else {
                    Remove-Item -Path Env:PATHEXT -ErrorAction SilentlyContinue
                }
            }

            $runExitCode | Should Not Be 0
            (Test-Path -LiteralPath $manifestPath -PathType Leaf) | Should Be $true
            (Get-Content -Raw -LiteralPath $manifestPath) | Should Be $existingManifest
            $validation = Invoke-TestValidator -Path $manifestPath
            $validation.ExitCode | Should Be 0
            @(Get-ChildItem -LiteralPath $outputPath -Filter ".dogfood-measurement-*.tmp" -Force).Count | Should Be 0
        }

        It "keeps mixed HTTP probe evidence unknown with counts and reasons" {
            $path = Join-Path $testRoot "mixed.json"
            Write-TestManifest -Path $path -Manifest (New-TestManifest)
            $result = Invoke-TestValidator -Path $path
            $result.ExitCode | Should Be 0

            $manifest = Get-Content -Raw -LiteralPath $path | ConvertFrom-Json
            $probe = $manifest.spec.measurements[0].spec
            $probe.requestCount | Should Be 5
            $probe.successCount | Should Be 4
            $probe.failureCount | Should Be 1
            $probe.failureReasons[0] | Should Be "http_status_503"
            $probe.status | Should Be "unknown"
        }

        It "rejects a partial probe mislabeled as pass" {
            $path = Join-Path $testRoot "partial-pass.json"
            $manifest = New-TestManifest
            $manifest.spec.measurements[0].spec.status = "pass"
            Write-TestManifest -Path $path -Manifest $manifest
            $result = Invoke-TestValidator -Path $path
            $result.ExitCode | Should Not Be 0
        }

        It "uses the canonical validator for unsafe measurement fields" {
            foreach ($field in @("name", "unit", "command")) {
                $path = Join-Path $testRoot ("unsafe-" + $field + ".json")
                $manifest = New-TestManifest
                if ($field -eq "name") {
                    $manifest.spec.measurements[0].spec.name = "C:\\unsafe\\metric"
                }
                elseif ($field -eq "unit") {
                    $manifest.spec.measurements[0].spec.unit = "milliseconds/second"
                }
                else {
                    $manifest.spec.measurements[0].spec.command = "GET C:\\unsafe\\endpoint"
                }
                Write-TestManifest -Path $path -Manifest $manifest
                $result = Invoke-TestValidator -Path $path
                $result.ExitCode | Should Not Be 0
            }
        }
    }
}
finally {
    if (Test-Path -LiteralPath $testRoot) {
        Remove-Item -LiteralPath $testRoot -Recurse -Force
    }
}
