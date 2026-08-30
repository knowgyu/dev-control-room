#requires -Version 7.6
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ManifestPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Assert-Condition {
    param(
        [bool]$Condition,
        [string]$Message
    )

    if (-not $Condition) {
        throw $Message
    }
}

function Assert-NoAbsolutePaths {
    param([object]$Value)

    if ($null -eq $Value) {
        return
    }
    if ($Value -is [string]) {
        Assert-Condition ($Value -notmatch "(?i)^(?:[A-Za-z]:[\\/]|[\\/]{1,2})") "manifest contains an absolute path"
        return
    }
    if ($Value -is [ValueType]) {
        return
    }
    if ($Value -is [System.Collections.IDictionary]) {
        foreach ($key in $Value.Keys) {
            Assert-NoAbsolutePaths -Value $Value[$key]
        }
        return
    }
    if ($Value -is [System.Collections.IEnumerable] -and $Value -isnot [string]) {
        foreach ($item in $Value) {
            Assert-NoAbsolutePaths -Value $item
        }
        return
    }
    foreach ($property in $Value.PSObject.Properties) {
        Assert-NoAbsolutePaths -Value $property.Value
    }
}

function Assert-FiniteSample {
    param([object]$Value)

    $number = [double]$Value
    Assert-Condition (-not [double]::IsNaN($number) -and -not [double]::IsInfinity($number)) "manifest contains a non-finite sample"
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

function Assert-SummaryValue {
    param(
        [object]$Actual,
        [double]$Expected,
        [string]$Field
    )

    Assert-FiniteSample -Value $Actual
    Assert-Condition ([math]::Abs(([double]$Actual) - $Expected) -le 0.001) ("measurement {0} summary is inconsistent" -f $Field)
}

$resolvedManifestPath = (Resolve-Path -LiteralPath $ManifestPath -ErrorAction Stop).ProviderPath
$manifestItem = Get-Item -LiteralPath $resolvedManifestPath -Force
Assert-Condition (-not $manifestItem.PSIsContainer -and -not ($manifestItem.Attributes -band [IO.FileAttributes]::ReparsePoint)) "manifest must be a regular file"
$manifest = Get-Content -Raw -LiteralPath $resolvedManifestPath | ConvertFrom-Json
Assert-NoAbsolutePaths -Value $manifest

Assert-Condition ([string]$manifest.apiVersion -eq "devroom/measurement/v1") "manifest API version is invalid"
Assert-Condition ([string]$manifest.kind -eq "DogfoodMeasurementRun") "manifest kind is invalid"
$spec = $manifest.spec
$reproducibility = $spec.reproducibility
Assert-Condition (-not [string]::IsNullOrWhiteSpace([string]$manifest.metadata.id)) "manifest run id is missing"
Assert-Condition ([string]$manifest.metadata.id -eq [string]$reproducibility.runId) "manifest run ids do not match"
Assert-Condition ([string]$spec.status -in @("pass", "fail", "unknown")) "manifest status is invalid"
Assert-Condition ([string]$reproducibility.dirtyState -in @("clean", "dirty", "unknown")) "manifest dirty state is invalid"
Assert-Condition ([string]$reproducibility.configurationDigest -match "^sha256:[0-9a-f]{64}$") "manifest configuration digest is invalid"
Assert-Condition (@($reproducibility.toolVersions.PSObject.Properties).Count -le 32) "manifest tool versions are unbounded"
$startedAt = [DateTimeOffset]::Parse([string]$reproducibility.startedAt)
$endedAt = [DateTimeOffset]::Parse([string]$reproducibility.endedAt)
Assert-Condition ($endedAt -ge $startedAt) "manifest time range is reversed"

$measurements = @($spec.measurements)
$measurementIDs = [System.Collections.Generic.List[string]]::new()
$derivedFailures = [System.Collections.Generic.List[string]]::new()
$requiredCount = 0
foreach ($measurement in $measurements) {
    Assert-Condition ([string]$measurement.apiVersion -eq "devroom/measurement/v1") "measurement API version is invalid"
    Assert-Condition ([string]$measurement.kind -eq "Measurement") "measurement kind is invalid"
    $id = [string]$measurement.metadata.id
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($id)) "measurement id is missing"
    Assert-Condition (-not $measurementIDs.Contains($id)) "measurement ids are not unique"
    [void]$measurementIDs.Add($id)
    $item = $measurement.spec
    Assert-Condition ([string]$item.category -in @("quality", "performance", "process", "runtime")) "measurement category is invalid"
    Assert-Condition ([string]$item.status -in @("pass", "fail", "unknown")) "measurement status is invalid"
    Assert-Condition ([string]$item.provenance -in @("measured", "estimated", "inferred", "unavailable")) "measurement provenance is invalid"
    Assert-Condition (-not [string]::IsNullOrWhiteSpace([string]$item.unit)) "measurement unit is missing"
    $samples = @($item.rawSamples)
    Assert-Condition ([int]$item.sampleCount -eq $samples.Count) "measurement sample count is inconsistent"
    Assert-Condition ($samples.Count -le 128) "measurement raw samples exceed the bounded contract"
    foreach ($sample in $samples) {
        Assert-FiniteSample -Value $sample
    }
    if ($samples.Count -eq 0) {
        Assert-Condition ([string]$item.status -eq "unknown") "zero-sample measurement is not unknown"
        Assert-Condition ([string]$item.provenance -ne "measured") "zero-sample measurement is marked measured"
        Assert-Condition ($null -eq $item.min -and $null -eq $item.p50 -and $null -eq $item.p95 -and $null -eq $item.max) "zero-sample measurement has summary values"
    }
    else {
        Assert-Condition ($null -ne $item.min -and $null -ne $item.p50 -and $null -ne $item.p95 -and $null -ne $item.max) "sampled measurement is missing a summary value"
        $sortedSamples = @($samples | Sort-Object)
        Assert-SummaryValue -Actual $item.min -Expected ([double]$sortedSamples[0]) -Field "min"
        Assert-SummaryValue -Actual $item.p50 -Expected (Get-PercentileValue -Samples $samples -Percentile 50) -Field "p50"
        Assert-SummaryValue -Actual $item.p95 -Expected (Get-PercentileValue -Samples $samples -Percentile 95) -Field "p95"
        Assert-SummaryValue -Actual $item.max -Expected ([double]$sortedSamples[-1]) -Field "max"
        Assert-Condition ([string]$item.provenance -ne "unavailable") "sampled measurement is marked unavailable"
    }
    if ($null -ne $item.baseline) { Assert-FiniteSample -Value $item.baseline }
    if ($null -ne $item.delta) { Assert-FiniteSample -Value $item.delta }
    if ([bool]$item.required) {
        $requiredCount++
        if ([string]$item.status -ne "pass") {
            [void]$derivedFailures.Add($id)
        }
    }
}

$expectedStatus = if ($requiredCount -eq 0) { "unknown" } elseif ($derivedFailures.Count -gt 0) { "fail" } else { "pass" }
Assert-Condition ([string]$spec.status -eq $expectedStatus) "manifest status does not match required measurements"
$declaredFailures = @($spec.requiredFailures | ForEach-Object { [string]$_ })
Assert-Condition ($declaredFailures.Count -eq $derivedFailures.Count) "manifest required failure count is inconsistent"
for ($index = 0; $index -lt $derivedFailures.Count; $index++) {
    Assert-Condition ($declaredFailures[$index] -eq $derivedFailures[$index]) "manifest required failure order is inconsistent"
}

Write-Host ("PASS measurement contract verified: {0} measurements" -f $measurements.Count)
