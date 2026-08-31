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

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).ProviderPath
$resolvedManifestPath = (Resolve-Path -LiteralPath $ManifestPath -ErrorAction Stop).ProviderPath
$manifestItem = Get-Item -LiteralPath $resolvedManifestPath -Force
Assert-Condition (-not $manifestItem.PSIsContainer -and -not ($manifestItem.Attributes -band [IO.FileAttributes]::ReparsePoint)) "manifest must be a regular file"
Assert-Condition ($manifestItem.Length -le (512 * 1024)) "manifest exceeds the bounded contract size"

$goCommand = Get-Command go -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
Assert-Condition ($null -ne $goCommand -and -not [string]::IsNullOrWhiteSpace([string]$goCommand.Source)) "canonical Go measurement validator is unavailable"
$goPath = [string]$goCommand.Source

$output = @()
$exitCode = 1
Push-Location $repositoryRoot
try {
    $output = @(& $goPath run ./cmd/verify-measurement-contract --manifest $resolvedManifestPath 2>&1)
    $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
}
finally {
    Pop-Location
}

if ($exitCode -ne 0) {
    $detail = @($output | ForEach-Object { [string]$_ } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }) -join " "
    if ([string]::IsNullOrWhiteSpace($detail)) {
        throw ("canonical Go measurement validator failed with exit code " + $exitCode)
    }
    throw ("canonical Go measurement validator failed: " + $detail)
}

foreach ($line in @($output)) {
    Write-Host ([string]$line)
}
