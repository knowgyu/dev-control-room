param(
    [string]$BinaryPath = "",
    [string]$ListenAddress = "127.0.0.1:38472"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Invoke-Native {
    param([string]$FilePath, [string[]]$Arguments)

    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath exited with code $LASTEXITCODE"
    }
}

function Invoke-JsonCommand {
    param([string]$FilePath, [string[]]$Arguments)

    $output = & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath exited with code $LASTEXITCODE"
    }
    return (($output -join "`n") | ConvertFrom-Json)
}

$repositoryRoot = Split-Path -Parent $PSScriptRoot
$artifactDirectory = Join-Path $repositoryRoot "artifacts"
if ($BinaryPath -eq "") {
    $BinaryPath = Join-Path $artifactDirectory "dev-control-room.exe"
}
if (-not (Test-Path -LiteralPath $BinaryPath)) {
    New-Item -ItemType Directory -Path $artifactDirectory -Force | Out-Null
    Push-Location $repositoryRoot
    try {
        Invoke-Native -FilePath "go" -Arguments @("build", "-trimpath", "-o", $BinaryPath, ".\cmd\dev-control-room")
    }
    finally {
        Pop-Location
    }
}
$BinaryPath = (Resolve-Path -LiteralPath $BinaryPath).ProviderPath

$version = (& $BinaryPath version).Trim()
if ($LASTEXITCODE -ne 0 -or $version -ne "0.4.0-rc.1") {
    throw "expected dev-control-room 0.4.0-rc.1, got '$version'"
}
$sourceCommit = (& git -c "safe.directory=$repositoryRoot" -C $repositoryRoot rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0) {
    throw "could not resolve source commit"
}

$acceptanceRoot = Join-Path ([IO.Path]::GetTempPath()) ("dev-control-room-rc-" + [guid]::NewGuid().ToString("N"))
$appDataRoot = Join-Path $acceptanceRoot "app-data"
$remoteRepository = Join-Path $acceptanceRoot "fixture-origin.git"
$fixtureRepository = Join-Path $acceptanceRoot "fixture-worktree"
New-Item -ItemType Directory -Path $appDataRoot -Force | Out-Null

Invoke-Native -FilePath "git" -Arguments @("init", "--bare", "--initial-branch=main", $remoteRepository)
Invoke-Native -FilePath "git" -Arguments @("clone", $remoteRepository, $fixtureRepository)
Invoke-Native -FilePath "git" -Arguments @("-C", $fixtureRepository, "config", "user.name", "Dev Control Room Fixture")
Invoke-Native -FilePath "git" -Arguments @("-C", $fixtureRepository, "config", "user.email", "fixture@example.invalid")
Set-Content -LiteralPath (Join-Path $fixtureRepository "README.md") -Value "# Dev Control Room native acceptance fixture" -Encoding utf8
Set-Content -LiteralPath (Join-Path $fixtureRepository "AGENTS.md") -Value "Use [README.md](README.md) as the fixture reference." -Encoding utf8
Invoke-Native -FilePath "git" -Arguments @("-C", $fixtureRepository, "add", "README.md", "AGENTS.md")
Invoke-Native -FilePath "git" -Arguments @("-C", $fixtureRepository, "commit", "-m", "fixture: initialize native acceptance")
Invoke-Native -FilePath "git" -Arguments @("-C", $fixtureRepository, "push", "-u", "origin", "main")

$projectEnvelope = Invoke-JsonCommand -FilePath $BinaryPath -Arguments @("project", "add", "--name", "RC Acceptance Fixture", "--path", $fixtureRepository, "--home", $appDataRoot, "--json")
Invoke-JsonCommand -FilePath $BinaryPath -Arguments @("project", "scan", "--home", $appDataRoot, "--json") | Out-Null
$projectId = $projectEnvelope.data.metadata.id
$repositoryId = $projectEnvelope.data.spec.repositories[0].metadata.id
$worktrees = Invoke-JsonCommand -FilePath $BinaryPath -Arguments @("project", "worktree", "list", $projectId, $repositoryId, "--home", $appDataRoot, "--json")
$worktreeId = $worktrees.data[0].metadata.id

Invoke-JsonCommand -FilePath $BinaryPath -Arguments @("agent", "profile", "add", "--id", "fixture-agent", "--name", "Fixture Agent", "--command", "git", "--version-probe=--version", "--timeout", "3", "--env", "PATH", "--launch-mode", "direct", "--data-boundary", "local", "--home", $appDataRoot, "--json") | Out-Null
$guidance = Invoke-JsonCommand -FilePath $BinaryPath -Arguments @("guidance", "check", $projectId, $repositoryId, $worktreeId, "--home", $appDataRoot, "--json")
$handoff = Invoke-JsonCommand -FilePath $BinaryPath -Arguments @("agent", "handoff", "preview", "--profile", "fixture-agent", "--project", $projectId, "--repository", $repositoryId, "--worktree", $worktreeId, "--model", "acceptance-fixture", "--home", $appDataRoot, "--json")
$cleanup = Invoke-JsonCommand -FilePath $BinaryPath -Arguments @("cleanup", "list", "--project", $projectId, "--home", $appDataRoot, "--json")

$mcpInput = @(
    '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}',
    '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
)
$mcpOutput = $mcpInput | & $BinaryPath mcp serve --home $appDataRoot
if ($LASTEXITCODE -ne 0 -or @($mcpOutput).Count -ne 2) {
    throw "MCP initialize/tools-list smoke failed"
}

$context = [ordered]@{
    sourceCommit = $sourceCommit
    binary = $BinaryPath
    acceptanceRoot = $acceptanceRoot
    appDataRoot = $appDataRoot
    fixtureRepository = $fixtureRepository
    projectId = $projectId
    repositoryId = $repositoryId
    worktreeId = $worktreeId
    listenAddress = $ListenAddress
}
$contextPath = Join-Path $acceptanceRoot "acceptance-context.json"
$context | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $contextPath -Encoding utf8
$guidance | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $acceptanceRoot "guidance.json") -Encoding utf8
$handoff | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $acceptanceRoot "handoff.json") -Encoding utf8
$cleanup | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $acceptanceRoot "cleanup.json") -Encoding utf8
$mcpOutput | Set-Content -LiteralPath (Join-Path $acceptanceRoot "mcp.jsonl") -Encoding utf8

$previousAppDataRoot = $env:DEV_CONTROL_ROOM_HOME
try {
    $env:DEV_CONTROL_ROOM_HOME = $appDataRoot
    $server = Start-Process -FilePath $BinaryPath -ArgumentList @("serve", "--listen", $ListenAddress) -PassThru
}
finally {
    if ($null -eq $previousAppDataRoot) {
        Remove-Item Env:DEV_CONTROL_ROOM_HOME -ErrorAction SilentlyContinue
    }
    else {
        $env:DEV_CONTROL_ROOM_HOME = $previousAppDataRoot
    }
}

$uri = "http://$ListenAddress/"
$ready = $false
for ($attempt = 0; $attempt -lt 20; $attempt++) {
    try {
        Invoke-WebRequest -Uri $uri -UseBasicParsing | Out-Null
        $ready = $true
        break
    }
    catch {
        Start-Sleep -Milliseconds 250
    }
}
if (-not $ready) {
    Stop-Process -Id $server.Id -ErrorAction SilentlyContinue
    throw "fixture server did not become ready at $uri"
}

$context["serverProcessId"] = $server.Id
$context | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $contextPath -Encoding utf8
$context | ConvertTo-Json -Depth 8
Write-Host "Open $uri and complete the Slice F UI/Action checklist."
Write-Host "Evidence is under $acceptanceRoot; no cleanup is performed automatically."
Write-Host "Stop only the fixture server with: Stop-Process -Id $($server.Id)"
