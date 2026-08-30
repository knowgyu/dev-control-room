#requires -Version 7.6
[CmdletBinding()]
param(
    [string]$OutputDirectory = "",
    [switch]$ProbeServer,
    [string]$ServerUri = "http://127.0.0.1:38471",
    [ValidateRange(1, 16)]
    [int]$RequestCount = 5,
    [ValidateRange(1, 10)]
    [int]$RequestTimeoutSeconds = 2
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).ProviderPath
$measurementAPIVersion = "devroom/measurement/v1"
$measurementRunKind = "DogfoodMeasurementRun"
$measurementKind = "Measurement"
$maxRawSamples = 128
$script:measurementList = [System.Collections.Generic.List[object]]::new()
$script:resultRows = [System.Collections.Generic.List[object]]::new()
$script:requiredFailures = [System.Collections.Generic.List[string]]::new()
$script:toolVersions = [ordered]@{}
$script:coverageProfileName = ""
$script:runnerFailure = $false
$script:startedAt = [DateTime]::UtcNow

function Test-PathWithin {
    param(
        [string]$Path,
        [string]$Root
    )

    $separators = [char[]]@([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
    $candidate = [IO.Path]::GetFullPath($Path).TrimEnd($separators)
    $rootPath = [IO.Path]::GetFullPath($Root).TrimEnd($separators)
    return $candidate.Equals($rootPath, [StringComparison]::OrdinalIgnoreCase) -or $candidate.StartsWith($rootPath + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)
}

function Get-OutputPath {
    if ([string]::IsNullOrWhiteSpace($OutputDirectory)) {
        return [IO.Path]::GetFullPath((Join-Path ([IO.Path]::GetTempPath()) ("dev-control-room-dogfood-" + [guid]::NewGuid().ToString("N"))))
    }
    if ([IO.Path]::IsPathRooted($OutputDirectory)) {
        return [IO.Path]::GetFullPath($OutputDirectory)
    }
    return [IO.Path]::GetFullPath((Join-Path $repositoryRoot $OutputDirectory))
}

function Assert-OutputDirectory {
    param([string]$Path)

    if ($Path.Length -gt 240) {
        throw "output directory path is too long"
    }
    $artifactsRoot = Join-Path $repositoryRoot "artifacts"
    if ((Test-PathWithin -Path $Path -Root $repositoryRoot) -and -not (Test-PathWithin -Path $Path -Root $artifactsRoot)) {
        throw "repository output must be under the ignored artifacts directory"
    }
    New-Item -ItemType Directory -Path $Path -Force | Out-Null
    $item = Get-Item -LiteralPath $Path -Force
    if (-not $item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw "output directory must be a regular directory"
    }
}

function Get-ServerOrigin {
    try {
        $uri = [Uri]::new($ServerUri)
    }
    catch {
        throw "server URI is invalid"
    }
    $allowedHosts = @("127.0.0.1", "localhost", "::1")
    $hostName = $uri.DnsSafeHost.ToLowerInvariant()
    $isAllowedHost = $allowedHosts -contains $hostName
    if (-not $uri.IsAbsoluteUri -or $uri.Scheme -ne "http" -or -not $isAllowedHost -or -not [string]::IsNullOrWhiteSpace($uri.UserInfo) -or -not [string]::IsNullOrWhiteSpace($uri.Query) -or -not [string]::IsNullOrWhiteSpace($uri.Fragment) -or ($uri.AbsolutePath -ne "/" -and $uri.AbsolutePath -ne "")) {
        throw "server URI must be an http loopback origin without credentials or extra paths"
    }
    $port = $uri.Port
    if ($port -lt 1 -or $port -gt 65535) {
        throw "server URI port is invalid"
    }
    if ($hostName -eq "::1") {
        return "http://[$hostName]:$port"
    }
    return "http://$hostName`:$port"
}

function Get-CommandPath {
    param([string]$Name)

    $command = Get-Command $Name -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $command -or [string]::IsNullOrWhiteSpace([string]$command.Source)) {
        return ""
    }
    return [string]$command.Source
}

function Invoke-ReadOnlyProcess {
    param(
        [string]$FilePath,
        [string[]]$Arguments = @(),
        [int]$MaxLines = 0
    )

    $lines = [System.Collections.Generic.List[string]]::new()
    $outputSeen = $false
    $overflow = $false
    $started = $false
    $exitCode = $null
    $timer = [Diagnostics.Stopwatch]::StartNew()
    try {
        if ([string]::IsNullOrWhiteSpace($FilePath)) {
            return [pscustomobject]@{ Started = $false; ExitCode = $null; OutputSeen = $false; Overflow = $false; Lines = @(); DurationMilliseconds = 0 }
        }
        $started = $true
        & $FilePath @Arguments 2>&1 | ForEach-Object {
            $outputSeen = $true
            if ($MaxLines -gt 0 -and $lines.Count -lt $MaxLines) {
                [void]$lines.Add([string]$_)
            }
            elseif ($MaxLines -gt 0) {
                $overflow = $true
            }
        }
        if ($null -eq $LASTEXITCODE) {
            $exitCode = 0
        }
        else {
            $exitCode = [int]$LASTEXITCODE
        }
    }
    catch {
        $exitCode = $null
    }
    finally {
        $timer.Stop()
    }
    return [pscustomobject]@{
        Started = $started
        ExitCode = $exitCode
        OutputSeen = $outputSeen
        Overflow = $overflow
        Lines = @($lines)
        DurationMilliseconds = [math]::Round($timer.Elapsed.TotalMilliseconds, 3)
    }
}

function Get-SafeToolVersion {
    param(
        [string]$FilePath,
        [string[]]$Arguments = @()
    )

    $result = Invoke-ReadOnlyProcess -FilePath $FilePath -Arguments $Arguments -MaxLines 1
    if (-not $result.Started -or $result.ExitCode -ne 0 -or @($result.Lines).Count -eq 0) {
        return "unavailable"
    }
    $value = ([string]$result.Lines[0]).Trim()
    if ([string]::IsNullOrWhiteSpace($value) -or $value.Length -gt 256 -or $value -match "[\x00-\x1F]" -or $value -match "(?i)^(?:[A-Za-z]:[\\/]|[\\/]{1,2})") {
        return "unavailable"
    }
    return $value
}

function Get-GitMetadata {
    param([string]$GitPath)

    $commit = "unavailable"
    $head = "unavailable"
    $dirtyState = "unknown"
    $headResult = Invoke-ReadOnlyProcess -FilePath $GitPath -Arguments @("-C", $repositoryRoot, "rev-parse", "--verify", "HEAD") -MaxLines 1
    if ($headResult.Started -and $headResult.ExitCode -eq 0 -and @($headResult.Lines).Count -eq 1 -and [string]$headResult.Lines[0] -match "^[0-9a-fA-F]{7,64}$") {
        $commit = ([string]$headResult.Lines[0]).Trim()
        $head = $commit
    }
    $statusResult = Invoke-ReadOnlyProcess -FilePath $GitPath -Arguments @("-C", $repositoryRoot, "status", "--porcelain", "--untracked-files=all")
    if ($statusResult.Started -and $statusResult.ExitCode -eq 0) {
        $dirtyState = if ($statusResult.OutputSeen) { "dirty" } else { "clean" }
    }
    return [pscustomobject]@{ Commit = $commit; Head = $head; DirtyState = $dirtyState }
}

function Get-RepositoryGoFiles {
    param([string]$GitPath)

    if ([string]::IsNullOrWhiteSpace($GitPath)) {
        return @()
    }
    $result = Invoke-ReadOnlyProcess -FilePath $GitPath -Arguments @("-C", $repositoryRoot, "ls-files", "--cached", "--others", "--exclude-standard", "--", "*.go") -MaxLines 4096
    if (-not $result.Started -or $result.ExitCode -ne 0 -or $result.Overflow) {
        return @()
    }
    $paths = [System.Collections.Generic.List[string]]::new()
    foreach ($relativePath in @($result.Lines)) {
        $relative = ([string]$relativePath).Trim()
        if ([string]::IsNullOrWhiteSpace($relative)) {
            continue
        }
        try {
            $fullPath = [IO.Path]::GetFullPath((Join-Path $repositoryRoot $relative))
            if (-not (Test-PathWithin -Path $fullPath -Root $repositoryRoot)) {
                return @()
            }
            $item = Get-Item -LiteralPath $fullPath -Force -ErrorAction Stop
            if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
                return @()
            }
            [void]$paths.Add($fullPath)
        }
        catch {
            return @()
        }
    }
    return @($paths | Sort-Object -Unique)
}

function Get-PercentileValue {
    param(
        [double[]]$Samples,
        [double]$Percentile
    )

    $sorted = @($Samples | Sort-Object)
    if ($sorted.Count -eq 1) {
        return [double]$sorted[0]
    }
    $position = $Percentile / 100 * ($sorted.Count - 1)
    $lower = [int][math]::Floor($position)
    $upper = [int][math]::Ceiling($position)
    if ($lower -eq $upper) {
        return [double]$sorted[$lower]
    }
    $fraction = $position - $lower
    return [double]$sorted[$lower] + (([double]$sorted[$upper] - [double]$sorted[$lower]) * $fraction)
}

function Get-SampleSummary {
    param([double[]]$Samples)

    $normalized = @($Samples | ForEach-Object { [math]::Round([double]$_, 3) })
    if ($normalized.Count -eq 0) {
        return [pscustomobject]@{ Min = $null; P50 = $null; P95 = $null; Max = $null; Samples = @() }
    }
    return [pscustomobject]@{
        Min = [math]::Round(([double]($normalized | Sort-Object)[0]), 3)
        P50 = [math]::Round((Get-PercentileValue -Samples $normalized -Percentile 50), 3)
        P95 = [math]::Round((Get-PercentileValue -Samples $normalized -Percentile 95), 3)
        Max = [math]::Round(([double]($normalized | Sort-Object)[-1]), 3)
        Samples = $normalized
    }
}

function New-MeasurementRecord {
    param(
        [string]$ID,
        [string]$Name,
        [string]$Category,
        [string]$Status,
        [string]$Provenance,
        [string]$Unit,
        [double[]]$Samples = @(),
        [object]$Baseline = $null,
        [object]$Delta = $null,
        [string]$CommandID = "",
        [string]$Command = "",
        [object]$ExitCode = $null,
        [bool]$Required = $false
    )

    $summary = Get-SampleSummary -Samples $Samples
    if (@($summary.Samples).Count -gt $maxRawSamples) {
        throw "measurement raw sample limit exceeded"
    }
    return [ordered]@{
        apiVersion = $measurementAPIVersion
        kind = $measurementKind
        metadata = [ordered]@{ id = $ID }
        spec = [ordered]@{
            name = $Name
            category = $Category
            status = $Status
            provenance = $Provenance
            unit = $Unit
            sampleCount = @($summary.Samples).Count
            rawSamples = @($summary.Samples)
            min = $summary.Min
            p50 = $summary.P50
            p95 = $summary.P95
            max = $summary.Max
            baseline = $Baseline
            delta = $Delta
            commandId = $CommandID
            command = $Command
            exitCode = $ExitCode
            required = $Required
        }
    }
}

function Add-ResultRow {
    param(
        [System.Collections.IDictionary]$Measurement,
        [string]$DisplayCommand,
        [int]$RequestCountValue = 0
    )

    $spec = $Measurement.spec
    [void]$script:resultRows.Add([pscustomobject]@{
        ID = [string]$Measurement.metadata.id
        Command = $DisplayCommand
        Status = [string]$spec.status
        Provenance = [string]$spec.provenance
        Unit = [string]$spec.unit
        Samples = [int]$spec.sampleCount
        P50 = $spec.p50
        P95 = $spec.p95
        ExitCode = $spec.exitCode
        Required = [bool]$spec.required
        RequestCount = $RequestCountValue
    })
}

function Add-Measurement {
    param(
        [System.Collections.IDictionary]$Measurement,
        [string]$DisplayCommand,
        [int]$RequestCountValue = 0
    )

    [void]$script:measurementList.Add($Measurement)
    Add-ResultRow -Measurement $Measurement -DisplayCommand $DisplayCommand -RequestCountValue $RequestCountValue
}

function Invoke-QualityCheck {
    param(
        [string]$ID,
        [string]$Name,
        [string]$CommandID,
        [string]$DisplayCommand,
        [string]$FilePath,
        [string[]]$Arguments = @(),
        [bool]$Required = $true,
        [bool]$FailOnOutput = $false
    )

    $result = Invoke-ReadOnlyProcess -FilePath $FilePath -Arguments $Arguments
    $samples = @()
    $status = "unknown"
    $provenance = "unavailable"
    $exitCode = $null
    if ($result.Started) {
        $samples = @([double]$result.DurationMilliseconds)
        $exitCode = $result.ExitCode
        if ($null -ne $result.ExitCode) {
            $status = if ($result.ExitCode -eq 0 -and (-not $FailOnOutput -or -not $result.OutputSeen)) { "pass" } else { "fail" }
            $provenance = "measured"
            if ($FailOnOutput -and $result.OutputSeen -and $result.ExitCode -eq 0) {
                $exitCode = 1
            }
        }
    }
    $measurement = New-MeasurementRecord -ID $ID -Name $Name -Category "quality" -Status $status -Provenance $provenance -Unit "milliseconds" -Samples $samples -CommandID $CommandID -Command $DisplayCommand -ExitCode $exitCode -Required $Required
    Add-Measurement -Measurement $measurement -DisplayCommand $DisplayCommand
    if ($Required -and $status -ne "pass") {
        [void]$script:requiredFailures.Add($ID)
    }
    return $measurement
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

function Invoke-Coverage {
    param(
        [string]$GoPath,
        [string[]]$GoFiles
    )

    $coverageName = "coverage-" + $script:runID + ".out"
    $coveragePath = Join-Path $outputPath $coverageName
    $script:coverageProfileName = $coverageName
    $coverageMeasurement = New-MeasurementRecord -ID "quality-go-coverage" -Name "quality.go.coverage" -Category "quality" -Status "unknown" -Provenance "unavailable" -Unit "milliseconds" -CommandID "go.coverage" -Command "go test -count=1 -coverprofile=coverage.out ./..." -Required $false
    Add-Measurement -Measurement $coverageMeasurement -DisplayCommand "go test -count=1 -coverprofile=coverage.out ./..."
    if ([string]::IsNullOrWhiteSpace($GoPath) -or @($GoFiles).Count -eq 0) {
        Add-Measurement -Measurement (New-MeasurementRecord -ID "quality-go-coverage-percent" -Name "quality.go.coverage_percent" -Category "quality" -Status "unknown" -Provenance "unavailable" -Unit "percent" -CommandID "go.coverage.summary" -Command "go tool cover -func=coverage.out" -Required $false) -DisplayCommand "go tool cover -func=coverage.out"
        return
    }
    $result = Invoke-ReadOnlyProcess -FilePath $GoPath -Arguments @("test", "-count=1", ("-coverprofile=" + $coveragePath), "./...")
    $samples = if ($result.Started) { @([double]$result.DurationMilliseconds) } else { @() }
    $status = "unknown"
    $provenance = "unavailable"
    if ($result.Started -and $null -ne $result.ExitCode) {
        $status = if ($result.ExitCode -eq 0) { "pass" } else { "fail" }
        $provenance = "measured"
    }
    $coverageMeasurement = New-MeasurementRecord -ID "quality-go-coverage" -Name "quality.go.coverage" -Category "quality" -Status $status -Provenance $provenance -Unit "milliseconds" -Samples $samples -CommandID "go.coverage" -Command "go test -count=1 -coverprofile=coverage.out ./..." -ExitCode $result.ExitCode -Required $false
    $script:measurementList.RemoveAt($script:measurementList.Count - 1)
    $script:resultRows.RemoveAt($script:resultRows.Count - 1)
    Add-Measurement -Measurement $coverageMeasurement -DisplayCommand "go test -count=1 -coverprofile=coverage.out ./..."

    $profileItem = if (Test-Path -LiteralPath $coveragePath -PathType Leaf) { Get-Item -LiteralPath $coveragePath -Force } else { $null }
    if ($status -ne "pass" -or $null -eq $profileItem -or ($profileItem.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        Add-Measurement -Measurement (New-MeasurementRecord -ID "quality-go-coverage-percent" -Name "quality.go.coverage_percent" -Category "quality" -Status "unknown" -Provenance "unavailable" -Unit "percent" -CommandID "go.coverage.summary" -Command "go tool cover -func=coverage.out" -Required $false) -DisplayCommand "go tool cover -func=coverage.out"
        return
    }
    $summaryResult = Invoke-ReadOnlyProcess -FilePath $GoPath -Arguments @("tool", "cover", ("-func=" + $coveragePath)) -MaxLines 4096
    $percent = $null
    foreach ($line in @($summaryResult.Lines)) {
        if ([string]$line -match "^\s*total:\s+\(statements\)\s+([0-9]+(?:\.[0-9]+)?)%") {
            $percent = [math]::Round([double]$matches[1], 3)
            break
        }
    }
    if ($summaryResult.Started -and $summaryResult.ExitCode -eq 0 -and $null -ne $percent) {
        Add-Measurement -Measurement (New-MeasurementRecord -ID "quality-go-coverage-percent" -Name "quality.go.coverage_percent" -Category "quality" -Status "pass" -Provenance "measured" -Unit "percent" -Samples @([double]$percent) -CommandID "go.coverage.summary" -Command "go tool cover -func=coverage.out" -Required $false) -DisplayCommand "go tool cover -func=coverage.out"
        return
    }
    Add-Measurement -Measurement (New-MeasurementRecord -ID "quality-go-coverage-percent" -Name "quality.go.coverage_percent" -Category "quality" -Status "unknown" -Provenance "unavailable" -Unit "percent" -CommandID "go.coverage.summary" -Command "go tool cover -func=coverage.out" -Required $false) -DisplayCommand "go tool cover -func=coverage.out"
}

function Invoke-ServerProbe {
    param(
        [string]$Origin,
        [string]$Path,
        [string]$ID,
        [string]$CommandID
    )

    $samples = [System.Collections.Generic.List[double]]::new()
    $badStatus = $false
    $handler = $null
    $client = $null
    try {
        $handler = [Net.Http.HttpClientHandler]::new()
        $handler.AllowAutoRedirect = $false
        $handler.UseProxy = $false
        $client = [Net.Http.HttpClient]::new($handler)
        $client.Timeout = [TimeSpan]::FromSeconds($RequestTimeoutSeconds)
        for ($index = 0; $index -lt $RequestCount; $index++) {
            $timer = [Diagnostics.Stopwatch]::StartNew()
            $response = $null
            try {
                $response = $client.GetAsync($Origin + $Path, [Net.Http.HttpCompletionOption]::ResponseHeadersRead).GetAwaiter().GetResult()
                $timer.Stop()
                [void]$samples.Add([math]::Round($timer.Elapsed.TotalMilliseconds, 3))
                if ([int]$response.StatusCode -lt 200 -or [int]$response.StatusCode -ge 300) {
                    $badStatus = $true
                }
            }
            catch {
                $timer.Stop()
                if ($samples.Count -eq 0) {
                    break
                }
            }
            finally {
                if ($null -ne $response) {
                    $response.Dispose()
                }
            }
        }
    }
    catch {
        $samples.Clear()
    }
    finally {
        if ($null -ne $client) {
            $client.Dispose()
        }
        if ($null -ne $handler) {
            $handler.Dispose()
        }
    }
    $status = "unknown"
    $provenance = "unavailable"
    if ($samples.Count -gt 0) {
        $status = if ($badStatus) { "fail" } else { "pass" }
        $provenance = "measured"
    }
    $measurement = New-MeasurementRecord -ID $ID -Name ("performance.http." + $CommandID + ".latency") -Category "performance" -Status $status -Provenance $provenance -Unit "milliseconds" -Samples @($samples) -CommandID ("http.get." + $CommandID) -Command ("GET " + $Path) -Required $false
    Add-Measurement -Measurement $measurement -DisplayCommand ("GET " + $Path) -RequestCountValue $RequestCount
}

function Add-UnavailableServerProbe {
    param(
        [string]$Path,
        [string]$ID,
        [string]$CommandID
    )

    $measurement = New-MeasurementRecord -ID $ID -Name ("performance.http." + $CommandID + ".latency") -Category "performance" -Status "unknown" -Provenance "unavailable" -Unit "milliseconds" -CommandID ("http.get." + $CommandID) -Command ("GET " + $Path + " (probe disabled)") -Required $false
    Add-Measurement -Measurement $measurement -DisplayCommand ("GET " + $Path)
}

function Format-ReportValue {
    param([object]$Value)

    if ($null -eq $Value) {
        return "unknown"
    }
    if ($Value -is [double] -or $Value -is [decimal] -or $Value -is [int] -or $Value -is [long]) {
        return ([double]$Value).ToString("0.###", [Globalization.CultureInfo]::InvariantCulture)
    }
    return [string]$Value
}

function Write-Report {
    param(
        [string]$Status,
        [string]$Commit,
        [string]$Head,
        [string]$DirtyState,
        [string]$ConfigurationDigest,
        [string]$EndedAt
    )

    $lines = [System.Collections.Generic.List[string]]::new()
    $markdownCode = [char]96
    [void]$lines.Add("# Dogfood measurement report")
    [void]$lines.Add("")
    [void]$lines.Add(("- Status: **{0}**" -f $Status.ToUpperInvariant()))
    [void]$lines.Add(("- Run ID: {0}{1}{0}" -f $markdownCode, $script:runID))
    [void]$lines.Add(("- Commit: {0}{1}{0}" -f $markdownCode, $Commit))
    [void]$lines.Add(("- Head: {0}{1}{0}" -f $markdownCode, $Head))
    [void]$lines.Add(("- Dirty state: {0}{1}{0}" -f $markdownCode, $DirtyState))
    [void]$lines.Add(("- Platform: {0}{1}/{2}{0}" -f $markdownCode, $script:os, $script:arch))
    [void]$lines.Add(("- Configuration digest: {0}{1}{0}" -f $markdownCode, $ConfigurationDigest))
    [void]$lines.Add(("- Started: {0}{1}{0}" -f $markdownCode, $script:startedAt.ToString("o")))
    [void]$lines.Add(("- Ended: {0}{1}{0}" -f $markdownCode, $EndedAt))
    [void]$lines.Add("")
    [void]$lines.Add("## Measurements")
    [void]$lines.Add("")
    [void]$lines.Add("| ID | Command | Required | Status | Provenance | Samples | P50 | P95 | Exit code |")
    [void]$lines.Add("| --- | --- | ---: | --- | --- | ---: | ---: | ---: | ---: |")
    foreach ($row in @($script:resultRows)) {
        $exitCode = if ($null -eq $row.ExitCode) { "unknown" } else { [string]$row.ExitCode }
        [void]$lines.Add(("| {0}{1}{0} | {0}{2}{0} | {3} | {0}{4}{0} | {0}{5}{0} | {6} | {7} | {8} | {9} |" -f $markdownCode, $row.ID, $row.Command, $row.Required, $row.Status, $row.Provenance, $row.Samples, (Format-ReportValue $row.P50), (Format-ReportValue $row.P95), $exitCode))
    }
    [void]$lines.Add("")
    $requiredFailureText = "none"
    if ($script:requiredFailures.Count -gt 0) {
        $requiredFailureText = @($script:requiredFailures) -join ", "
    }
    [void]$lines.Add("Required failures: " + $requiredFailureText)
    [void]$lines.Add("")
    [void]$lines.Add(("The JSON manifest preserves bounded raw samples. {0}unknown{0} and {0}unavailable{0} are not passes; they mean that comparable evidence was not obtained. Command output is intentionally not embedded in this report." -f $markdownCode))
    if (-not [string]::IsNullOrWhiteSpace($script:coverageProfileName)) {
        [void]$lines.Add(("Coverage profile artifact: {0}{1}{0}" -f $markdownCode, $script:coverageProfileName))
    }
    [IO.File]::WriteAllLines($reportPath, $lines, [Text.UTF8Encoding]::new($false))
}

function Write-Manifest {
    param(
        [string]$Status,
        [string]$Commit,
        [string]$Head,
        [string]$DirtyState,
        [string]$ConfigurationDigest,
        [string]$EndedAt
    )

    $manifest = [ordered]@{
        apiVersion = $measurementAPIVersion
        kind = $measurementRunKind
        metadata = [ordered]@{ id = $script:runID }
        spec = [ordered]@{
            status = $Status
            requiredFailures = @($script:requiredFailures)
            reproducibility = [ordered]@{
                runId = $script:runID
                commit = $Commit
                head = $Head
                dirtyState = $DirtyState
                os = $script:os
                arch = $script:arch
                toolVersions = $script:toolVersions
                configurationDigest = $ConfigurationDigest
                startedAt = $script:startedAt.ToString("o")
                endedAt = $EndedAt
            }
            measurements = @($script:measurementList)
        }
    }
    $json = $manifest | ConvertTo-Json -Depth 20
    [IO.File]::WriteAllText($manifestPath, $json, [Text.UTF8Encoding]::new($false))
}

$outputPath = Get-OutputPath
Assert-OutputDirectory -Path $outputPath
$manifestPath = Join-Path $outputPath "dogfood-measurement.json"
$reportPath = Join-Path $outputPath "dogfood-measurement-report.md"
$serverOrigin = Get-ServerOrigin
$script:runID = "dogfood-" + [guid]::NewGuid().ToString("N")
$script:os = if ([System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)) { "windows" } elseif ([System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Linux)) { "linux" } else { "other" }
$script:arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
$gitPath = Get-CommandPath "git"
$goPath = Get-CommandPath "go"
$gofmtPath = Get-CommandPath "gofmt"
$nodePath = Get-CommandPath "node"
$gitMetadata = Get-GitMetadata -GitPath $gitPath
$script:toolVersions.git = Get-SafeToolVersion -FilePath $gitPath -Arguments @("--version")
$script:toolVersions.go = Get-SafeToolVersion -FilePath $goPath -Arguments @("version")
$script:toolVersions.gofmt = if ([string]::IsNullOrWhiteSpace($gofmtPath)) { "unavailable" } else { $script:toolVersions.go }
$script:toolVersions.node = Get-SafeToolVersion -FilePath $nodePath -Arguments @("--version")
$script:toolVersions.powershell = "PowerShell " + $PSVersionTable.PSVersion.ToString()
$fixedChecks = @(
    [ordered]@{ id = "gofmt"; command = "gofmt -l [repository Go files]"; required = $true },
    [ordered]@{ id = "go-test"; command = "go test -count=1 ./..."; required = $true },
    [ordered]@{ id = "go-test-race"; command = "go test -race ./..."; required = $true },
    [ordered]@{ id = "go-vet"; command = "go vet ./..."; required = $true },
    [ordered]@{ id = "go-mod-verify"; command = "go mod verify"; required = $true },
    [ordered]@{ id = "go-build"; command = "go build ./..."; required = $true },
    [ordered]@{ id = "ui-syntax"; command = "node --check internal/app/ui/app.js"; required = $true }
)
$measurementConfig = [ordered]@{
    version = "dogfood-measurement-config/v1"
    rawSampleLimit = $maxRawSamples
    checks = $fixedChecks
    coverage = [ordered]@{ enabled = $true; command = "go test -count=1 -coverprofile=coverage.out ./..."; required = $false }
    server = [ordered]@{ enabled = [bool]$ProbeServer; origin = $serverOrigin; requestCount = $RequestCount; timeoutSeconds = $RequestTimeoutSeconds; endpoints = @("/api/health", "/api/state") }
}
$configurationJson = $measurementConfig | ConvertTo-Json -Depth 12 -Compress
$configurationDigest = "sha256:" + ([Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($configurationJson))).ToLowerInvariant())
$goFiles = Get-RepositoryGoFiles -GitPath $gitPath

try {
    Invoke-QualityCheck -ID "quality-gofmt" -Name "quality.gofmt" -CommandID "gofmt.check" -DisplayCommand "gofmt -l [repository Go files]" -FilePath $gofmtPath -Arguments (@("-l") + @($goFiles)) -Required $true -FailOnOutput ($goFiles.Count -gt 0) | Out-Null
    Invoke-QualityCheck -ID "quality-go-test" -Name "quality.go.test" -CommandID "go.test" -DisplayCommand "go test -count=1 ./..." -FilePath $goPath -Arguments @("test", "-count=1", "./...") -Required $true | Out-Null
    Invoke-WithEnvironment @{ CGO_ENABLED = "1" } {
        Invoke-QualityCheck -ID "quality-go-test-race" -Name "quality.go.test_race" -CommandID "go.test.race" -DisplayCommand "go test -race ./..." -FilePath $goPath -Arguments @("test", "-race", "./...") -Required $true | Out-Null
    }
    Invoke-QualityCheck -ID "quality-go-vet" -Name "quality.go.vet" -CommandID "go.vet" -DisplayCommand "go vet ./..." -FilePath $goPath -Arguments @("vet", "./...") -Required $true | Out-Null
    Invoke-QualityCheck -ID "quality-go-mod-verify" -Name "quality.go.mod_verify" -CommandID "go.mod.verify" -DisplayCommand "go mod verify" -FilePath $goPath -Arguments @("mod", "verify") -Required $true | Out-Null
    Invoke-QualityCheck -ID "quality-go-build" -Name "quality.go.build" -CommandID "go.build" -DisplayCommand "go build ./..." -FilePath $goPath -Arguments @("build", "./...") -Required $true | Out-Null
    Invoke-QualityCheck -ID "quality-ui-syntax" -Name "quality.ui.syntax" -CommandID "node.check.ui" -DisplayCommand "node --check internal/app/ui/app.js" -FilePath $nodePath -Arguments @("--check", "internal/app/ui/app.js") -Required $true | Out-Null
    Invoke-Coverage -GoPath $goPath -GoFiles $goFiles
    if ($ProbeServer) {
        Invoke-ServerProbe -Origin $serverOrigin -Path "/api/health" -ID "performance-http-health" -CommandID "health"
        Invoke-ServerProbe -Origin $serverOrigin -Path "/api/state" -ID "performance-http-state" -CommandID "state"
    }
    else {
        Add-UnavailableServerProbe -Path "/api/health" -ID "performance-http-health" -CommandID "health"
        Add-UnavailableServerProbe -Path "/api/state" -ID "performance-http-state" -CommandID "state"
    }
}
catch {
    $script:runnerFailure = $true
    Write-Host "Runner did not complete every measurement; the exported result is fail-closed."
}

$endedAt = [DateTime]::UtcNow
$runStatus = if ($script:requiredFailures.Count -eq 0 -and -not $script:runnerFailure) { "pass" } else { "fail" }
$runDuration = [math]::Round(($endedAt - $script:startedAt).TotalMilliseconds, 3)
Add-Measurement -Measurement (New-MeasurementRecord -ID "process-dogfood-run" -Name "process.dogfood.run_duration" -Category "process" -Status $runStatus -Provenance "measured" -Unit "milliseconds" -Samples @([double]$runDuration) -CommandID "runner" -Command "measure-dogfood.ps1" -Required $false) -DisplayCommand "measure-dogfood.ps1"
Write-Manifest -Status $runStatus -Commit $gitMetadata.Commit -Head $gitMetadata.Head -DirtyState $gitMetadata.DirtyState -ConfigurationDigest $configurationDigest -EndedAt $endedAt.ToString("o")
Write-Report -Status $runStatus -Commit $gitMetadata.Commit -Head $gitMetadata.Head -DirtyState $gitMetadata.DirtyState -ConfigurationDigest $configurationDigest -EndedAt $endedAt.ToString("o")
Write-Host ("Status: " + $runStatus.ToUpperInvariant())
Write-Host ("Manifest: " + [IO.Path]::GetFileName($manifestPath))
Write-Host ("Report: " + [IO.Path]::GetFileName($reportPath))
if ($runStatus -ne "pass") {
    exit 1
}
