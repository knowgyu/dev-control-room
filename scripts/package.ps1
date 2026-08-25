[CmdletBinding()]
param(
    [string]$Version = "0.5.0",
    [string]$OutputDirectory = "artifacts\0.5.0"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).ProviderPath
$outputRoot = if ([IO.Path]::IsPathRooted($OutputDirectory)) { $OutputDirectory } else { Join-Path $repositoryRoot $OutputDirectory }
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
        Copy-Item (Join-Path $repositoryRoot "docs\NATIVE_WINDOWS_SMOKE.md"), (Join-Path $repositoryRoot "docs\VERIFICATION_PLAYBOOK.md") -Destination (Join-Path $stage "docs")
        $zipPath = Join-Path $outputRoot ($name + ".zip")
        if (Test-Path -LiteralPath $zipPath) { Remove-Item -LiteralPath $zipPath -Force }
        Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zipPath
    }
    Get-ChildItem -LiteralPath $outputRoot -Filter "*.zip" | Get-FileHash -Algorithm SHA256 |
        ForEach-Object { "{0}  {1}" -f $_.Hash.ToLowerInvariant(), $_.Path.Substring($outputRoot.Length + 1) } |
        Set-Content -LiteralPath (Join-Path $outputRoot "SHA256SUMS") -Encoding ascii
    Write-Host "Packages: $outputRoot"
}
finally {
    if (Test-Path -LiteralPath $stagingRoot) { Remove-Item -LiteralPath $stagingRoot -Recurse -Force }
}
