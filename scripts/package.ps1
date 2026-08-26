[CmdletBinding()]
param(
    [string]$Version = "0.8.0",
    [string]$OutputDirectory
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).ProviderPath
if ([string]::IsNullOrWhiteSpace($Version) -or $Version -notmatch '^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z][0-9A-Za-z.-]*)?$') {
    throw "-Version must be a numeric semantic version, for example 0.7.0."
}
$releaseNotesPath = Join-Path $repositoryRoot ("docs\RELEASE_NOTES_v{0}.md" -f $Version)
$verificationPath = Join-Path $repositoryRoot ("docs\VERIFICATION_v{0}.md" -f $Version)
foreach ($expectedDocument in @($releaseNotesPath, $verificationPath)) {
    if (-not (Test-Path -LiteralPath $expectedDocument -PathType Leaf)) {
        throw ("Expected release document for version {0} was not found: {1}" -f $Version, $expectedDocument)
    }
}
$outputRoot = if ([IO.Path]::IsPathRooted($OutputDirectory)) {
    [IO.Path]::GetFullPath($OutputDirectory)
}
elseif ([string]::IsNullOrWhiteSpace($OutputDirectory)) {
    [IO.Path]::GetFullPath((Join-Path $repositoryRoot ("artifacts\{0}" -f $Version)))
}
else {
    [IO.Path]::GetFullPath((Join-Path $repositoryRoot $OutputDirectory))
}
$stagingRoot = Join-Path ([IO.Path]::GetTempPath()) ("dev-control-room-package-" + [guid]::NewGuid().ToString("N"))

try {
    New-Item -ItemType Directory -Path $outputRoot -Force | Out-Null
    foreach ($architecture in @("amd64", "arm64")) {
        $name = "dev-control-room_${Version}_windows_${architecture}"
        $stage = Join-Path $stagingRoot $name
        New-Item -ItemType Directory -Path $stage -Force | Out-Null
        Push-Location $repositoryRoot
        try {
            $env:GOOS = "windows"
            $env:GOARCH = $architecture
            $env:CGO_ENABLED = "0"
            & go build -trimpath -o (Join-Path $stage "dev-control-room.exe") .\cmd\dev-control-room
            if ($LASTEXITCODE -ne 0) { throw "Go build failed for $architecture" }
        }
        finally {
            Pop-Location
            Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
        }
        Copy-Item (Join-Path $repositoryRoot "README.md"), (Join-Path $repositoryRoot "LICENSE"), (Join-Path $repositoryRoot "THIRD_PARTY_POLICY.md") -Destination $stage
        New-Item -ItemType Directory -Path (Join-Path $stage "docs") -Force | Out-Null
        Copy-Item (Join-Path $repositoryRoot "docs\NATIVE_WINDOWS_SMOKE.md"), (Join-Path $repositoryRoot "docs\VERIFICATION_PLAYBOOK.md"), $releaseNotesPath, $verificationPath -Destination (Join-Path $stage "docs")
        $zipPath = Join-Path $outputRoot ($name + ".zip")
        if (Test-Path -LiteralPath $zipPath) { Remove-Item -LiteralPath $zipPath -Force }
        Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zipPath
    }
    Get-ChildItem -LiteralPath $outputRoot -Filter "*.zip" | Get-FileHash -Algorithm SHA256 |
        ForEach-Object { "{0}  {1}" -f $_.Hash.ToLowerInvariant(), ([IO.Path]::GetFileName($_.Path)) } |
        Set-Content -LiteralPath (Join-Path $outputRoot "SHA256SUMS") -Encoding ascii
    Write-Host "Packages: $outputRoot"
}
finally {
    if (Test-Path -LiteralPath $stagingRoot) { Remove-Item -LiteralPath $stagingRoot -Recurse -Force }
}
