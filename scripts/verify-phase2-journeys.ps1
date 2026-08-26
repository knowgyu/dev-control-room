[CmdletBinding()]
param(
    [string]$BinaryPath = "",
    [ValidateRange(0, 65535)]
    [int]$Port = 0,
    [switch]$KeepTemp
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).ProviderPath
$runtimeRoot = Join-Path ([IO.Path]::GetTempPath()) ("dev-control-room-phase2-journey-" + [guid]::NewGuid().ToString("N"))
$appHome = Join-Path $runtimeRoot "app-home"
$fixturePath = Join-Path $runtimeRoot "git-fixture-one"
$fixturePathTwo = Join-Path $runtimeRoot "git-fixture-two"
$processOutputRoot = Join-Path $runtimeRoot "process-output"
$codexFixtureRoot = Join-Path $runtimeRoot "codex-npm-fixture"
$codexBinPath = Join-Path $codexFixtureRoot "bin"
$codexPackageRoot = Join-Path $codexBinPath "node_modules\@openai\codex"
$codexNodePath = Join-Path $codexBinPath "node.exe"
$codexLauncherPath = Join-Path $codexBinPath "codex.cmd"
$serverProcess = $null
$serverLogPath = ""
$serverErrorPath = ""
$script:processCount = 0
$script:passCount = 0
$script:gitPath = ""
$script:appBinary = ""
$script:listenAddress = ""
$script:serverProcess = $null
$script:mutationToken = ""
$script:previousPath = [Environment]::GetEnvironmentVariable("Path", "Process")
$script:codexNodePath = $codexNodePath
$script:codexLauncherPath = $codexLauncherPath
$script:codexPackageRoot = $codexPackageRoot
$script:secretName = "PHASE2_SECRET_CANARY"
$script:secretValue = "phase2-secret-canary-" + [guid]::NewGuid().ToString("N")
$script:previousSecretValue = [Environment]::GetEnvironmentVariable($script:secretName, "Process")

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

function Assert-IsolatedTempRoot {
    $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
    $targetRoot = [IO.Path]::GetFullPath($runtimeRoot).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
    $targetPrefix = $tempRoot + [IO.Path]::DirectorySeparatorChar
    Assert-Condition ($targetRoot.StartsWith($targetPrefix, [StringComparison]::OrdinalIgnoreCase) -and $targetRoot -ne $tempRoot) "temporary journey root is an isolated child of the OS temp directory"
}

function Write-Utf8File {
    param(
        [string]$Path,
        [string]$Content
    )

    [IO.File]::WriteAllText($Path, $Content, [Text.UTF8Encoding]::new($false))
}

function Quote-ProcessArgument {
    param([AllowEmptyString()][string]$Value)

    if ($Value -notmatch '[\s"]') {
        return $Value
    }
    return '"' + $Value.Replace('"', '\"') + '"'
}

function Invoke-ProcessChecked {
    param(
        [string]$Name,
        [string]$FilePath,
        [string[]]$Arguments = @(),
        [string]$WorkingDirectory = $repositoryRoot,
        [switch]$AllowFailure
    )

    $script:processCount++
    $index = $script:processCount
    $stdoutPath = Join-Path $processOutputRoot ("{0:D3}-{1}.stdout.txt" -f $index, $Name)
    $stderrPath = Join-Path $processOutputRoot ("{0:D3}-{1}.stderr.txt" -f $index, $Name)
    $argumentText = (($Arguments | ForEach-Object { Quote-ProcessArgument ([string]$_) }) -join " ")
    $commandText = (Quote-ProcessArgument $FilePath) + $(if ($argumentText) { " " + $argumentText } else { "" })
    Write-Host (">>> " + $commandText)

    $process = Start-Process -FilePath $FilePath -ArgumentList $argumentText -WorkingDirectory $WorkingDirectory `
        -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath -WindowStyle Hidden -PassThru
    $process.WaitForExit()
    $exitCode = [int]$process.ExitCode
    $stdout = if (Test-Path -LiteralPath $stdoutPath) { Get-Content -Raw -LiteralPath $stdoutPath } else { "" }
    $stderr = if (Test-Path -LiteralPath $stderrPath) { Get-Content -Raw -LiteralPath $stderrPath } else { "" }
    if ($exitCode -ne 0 -and -not $AllowFailure) {
        throw ("{0} failed with exit code {1}. stderr: {2}" -f $Name, $exitCode, $stderr.Trim())
    }
    return [pscustomobject]@{
        Name = $Name
        Command = $commandText
        ExitCode = $exitCode
        Stdout = $stdout
        Stderr = $stderr
    }
}

function Invoke-AppCliJson {
    param(
        [string[]]$Arguments,
        [switch]$AllowFailure
    )

    $cliArguments = @($Arguments)
    $isHelp = $cliArguments.Count -gt 0 -and $cliArguments[0] -in @("--help", "-h", "help")
    if (-not $isHelp) {
        $cliArguments += @("--home", $appHome)
    }
    $cliArguments += "--json"
    if ($AllowFailure) {
        $result = Invoke-ProcessChecked -Name ("cli-" + ($Arguments[0] -replace '[^a-zA-Z0-9_-]', '_')) `
            -FilePath $script:appBinary -Arguments $cliArguments -AllowFailure
    }
    else {
        $result = Invoke-ProcessChecked -Name ("cli-" + ($Arguments[0] -replace '[^a-zA-Z0-9_-]', '_')) `
            -FilePath $script:appBinary -Arguments $cliArguments
    }
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($result.Stdout)) ("CLI returned JSON for " + ($Arguments -join " "))
    try {
        $envelope = $result.Stdout | ConvertFrom-Json
    }
    catch {
        throw ("CLI output was not JSON for {0}: {1}" -f ($Arguments -join " "), $result.Stdout.Trim())
    }
    Assert-Condition ([bool]$envelope.ok) ("CLI envelope is OK for " + ($Arguments -join " "))
    return $envelope.data
}

function Invoke-AppCliExpectedFailure {
    param(
        [string[]]$Arguments,
        [string]$ExpectedText
    )

    $cliArguments = @($Arguments) + @("--home", $appHome, "--json")
    $result = Invoke-ProcessChecked -Name ("cli-expected-failure-" + ($Arguments[0] -replace '[^a-zA-Z0-9_-]', '_')) `
        -FilePath $script:appBinary -Arguments $cliArguments -AllowFailure
    Assert-Condition ($result.ExitCode -ne 0) ("CLI blocks the unsafe action: " + ($Arguments -join " "))
    $combined = $result.Stdout + "`n" + $result.Stderr
    Assert-Condition ($combined -match $ExpectedText) ("blocked CLI explains the required recovery: " + $ExpectedText)
    return $result
}

function Invoke-LoopbackJson {
    param([string]$Path)

    $uri = "http://$($script:listenAddress)$Path"
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $uri -TimeoutSec 10
    }
    catch {
        throw ("loopback request failed for {0}: {1}" -f $Path, $_.Exception.Message)
    }
    Assert-Condition ($response.StatusCode -eq 200) ("HTTP 200 for " + $Path)
    try {
        $envelope = $response.Content | ConvertFrom-Json
    }
    catch {
        throw ("loopback response was not JSON for {0}: {1}" -f $Path, $response.Content)
    }
    Assert-Condition ([bool]$envelope.ok) ("HTTP envelope is OK for " + $Path)
    return $envelope.data
}

function Get-LoopbackMutationToken {
    $response = Invoke-WebRequest -UseBasicParsing -Uri ("http://$($script:listenAddress)/") -TimeoutSec 10
    $match = [regex]::Match($response.Content, '<meta\s+name="control-room-token"\s+content="([^"]+)"')
    Assert-Condition ($match.Success -and -not [string]::IsNullOrWhiteSpace($match.Groups[1].Value)) "loopback UI exposes a transient mutation token"
    return $match.Groups[1].Value
}

function Invoke-LoopbackMutationJson {
    param(
        [string]$Method,
        [string]$Path,
        [object]$Body
    )

    if ([string]::IsNullOrWhiteSpace($script:mutationToken)) {
        $script:mutationToken = Get-LoopbackMutationToken
    }
    $jsonBody = $Body | ConvertTo-Json -Depth 20 -Compress
    $headers = @{ "X-Control-Room-Token" = $script:mutationToken }
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Method $Method -Uri ("http://$($script:listenAddress)$Path") `
            -Headers $headers -ContentType "application/json" -Body $jsonBody -TimeoutSec 10
    }
    catch {
        throw ("loopback mutation failed for {0}: {1}" -f $Path, $_.Exception.Message)
    }
    Assert-Condition ($response.StatusCode -in @(200, 201)) ("HTTP mutation succeeded for " + $Path)
    try {
        $envelope = $response.Content | ConvertFrom-Json
    }
    catch {
        throw ("loopback mutation response was not JSON for {0}: {1}" -f $Path, $response.Content)
    }
    Assert-Condition ([bool]$envelope.ok) ("HTTP mutation envelope is OK for " + $Path)
    return $envelope.data
}

function Get-FreeLoopbackPort {
    param([int]$RequestedPort)

    if ($RequestedPort -ne 0) {
        return $RequestedPort
    }
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    try {
        $listener.Start()
        return ([Net.IPEndPoint]$listener.LocalEndpoint).Port
    }
    finally {
        $listener.Stop()
    }
}

function Invoke-GitChecked {
    param([string[]]$Arguments)

    $output = @(& $script:gitPath @Arguments 2>&1)
    $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
    if ($exitCode -ne 0) {
        throw ("git fixture command failed ({0}): {1}" -f ($Arguments -join " "), ($output -join " "))
    }
}

function Initialize-SafeGitRepository {
    param(
        [string]$Path,
        [string]$Name,
        [string]$ModulePath
    )

    New-Item -ItemType Directory -Path $Path -Force | Out-Null
    Invoke-GitChecked @("init", "--quiet", $Path)
    Invoke-GitChecked @("-C", $Path, "config", "user.name", "Phase 2 Journey Fixture")
    Invoke-GitChecked @("-C", $Path, "config", "user.email", "phase2-journey@example.invalid")
    Invoke-GitChecked @("-C", $Path, "branch", "-M", "main")
    Write-Utf8File (Join-Path $Path "README.md") ("# {0}`n" -f $Name)
    Write-Utf8File (Join-Path $Path "go.mod") ("module {0}`ngo 1.23`n" -f $ModulePath)
    Write-Utf8File (Join-Path $Path "main.go") "package main`n`nfunc main() {}`n"
    New-Item -ItemType Directory -Path (Join-Path $Path ".github\workflows") -Force | Out-Null
    Write-Utf8File (Join-Path $Path ".github\required-checks.txt") "go vet`n"
    Write-Utf8File (Join-Path $Path ".github\workflows\ci.yml") "name: local-ci`njobs:`n  verify:`n    steps:`n      - run: go vet ./...`n"
    Invoke-GitChecked @("-C", $Path, "add", "--", "README.md", "go.mod", "main.go", ".github")
    Invoke-GitChecked @("-C", $Path, "commit", "--quiet", "-m", "initial fixture commit")
    $status = @(& $script:gitPath -C $Path status --short 2>&1)
    $statusCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
    Assert-Condition ($statusCode -eq 0 -and $status.Count -eq 0) ("temporary Git fixture is clean: " + $Name)
}

function New-SafeGitFixture {
    Initialize-SafeGitRepository -Path $fixturePath -Name "Phase 2 journey repository one" -ModulePath "example.invalid/phase2-journey-one"
    Initialize-SafeGitRepository -Path $fixturePathTwo -Name "Phase 2 journey repository two" -ModulePath "example.invalid/phase2-journey-two"
    Assert-Condition ((Test-Path -LiteralPath $fixturePath -PathType Container) -and (Test-Path -LiteralPath $fixturePathTwo -PathType Container)) "fresh journey fixture contains two independent repositories"
}

function New-SyntheticCodexNpmLayout {
    New-Item -ItemType Directory -Path $codexPackageRoot -Force | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $codexPackageRoot "bin") -Force | Out-Null
    $nodeCommand = Get-Command node -CommandType Application -ErrorAction Stop | Select-Object -First 1
    Assert-Condition (-not [string]::IsNullOrWhiteSpace([string]$nodeCommand.Source)) "a native node.exe is available for the local launcher fixture"
    Assert-Condition ([IO.Path]::GetFileName($nodeCommand.Source) -ieq "node.exe") "the synthetic launcher fixture copies node.exe, not a shell shim"
    Copy-Item -LiteralPath $nodeCommand.Source -Destination $codexNodePath -Force
    Assert-Condition (Test-Path -LiteralPath $codexNodePath -PathType Leaf) "synthetic local node.exe was copied under the isolated fixture root"

    Write-Utf8File $codexLauncherPath "This npm shim is a discovery anchor only. It must never be executed.`n"
    Write-Utf8File (Join-Path $codexPackageRoot "package.json") @'
{
  "name": "@openai/codex",
  "bin": {
    "codex": "bin/codex.js"
  }
}
'@
    Write-Utf8File (Join-Path $codexPackageRoot "bin\codex.js") @'
const fs = require("fs");
const args = process.argv.slice(2);
const required = [
  "exec", "--json", "--sandbox", "read-only", "--ephemeral",
  "--ignore-user-config", "--ignore-rules", "--cd"
];
for (let index = 0; index < required.length; index += 1) {
  if (args[index] !== required[index]) {
    process.stderr.write("synthetic Codex received unexpected typed argv\n");
    process.exit(2);
  }
}
if (args[9] !== "--output-schema" || args[11] !== "--model" || args[13] !== "--" || args.length !== 15) {
  process.stderr.write("synthetic Codex received incomplete typed argv\n");
  process.exit(3);
}
if (args.some((value) => /(?:cmd\.exe|\.cmd|\.bat)$/i.test(value))) {
  process.stderr.write("synthetic Codex refuses shell launchers\n");
  process.exit(4);
}
fs.writeFileSync(args[10] + ".synthetic-run", "node.exe\n", { encoding: "utf8" });
process.stdout.write(JSON.stringify({ type: "thread.started", thread_id: "phase2-synthetic" }) + "\n");
process.stdout.write(JSON.stringify({
  type: "item.completed",
  item: {
    type: "agent_message",
    result: {
      summary: "합성 Codex launcher를 node.exe로 실행했습니다.",
      findings: [],
      nextAction: "검토 결과를 확인합니다."
    }
  }
}) + "\n");
process.stdout.write(JSON.stringify({
  type: "turn.completed",
  usage: { input_tokens: 17, output_tokens: 9, total_tokens: 26 }
}) + "\n");
'@

    $script:codexNodePath = $codexNodePath
    $script:codexLauncherPath = $codexLauncherPath
    $script:codexPackageRoot = $codexPackageRoot
    $previous = $script:previousPath
    if ([string]::IsNullOrWhiteSpace($previous)) {
        $env:Path = $codexBinPath
    }
    else {
        $env:Path = $codexBinPath + ";" + $previous
    }
    $resolvedNode = Get-Command node -CommandType Application -ErrorAction Stop | Select-Object -First 1
    $resolvedLauncher = Get-Command codex.cmd -CommandType Application -ErrorAction Stop | Select-Object -First 1
    Assert-Condition ($resolvedNode.Source -ieq $codexNodePath) "PATH resolves the synthetic node.exe"
    Assert-Condition ($resolvedLauncher.Source -ieq $codexLauncherPath) "PATH exposes the npm codex.cmd anchor for discovery"
    Assert-Condition (Test-Path -LiteralPath (Join-Path $codexPackageRoot "package.json") -PathType Leaf) "synthetic @openai/codex package metadata exists"
    Assert-Condition (Test-Path -LiteralPath (Join-Path $codexPackageRoot "bin\codex.js") -PathType Leaf) "synthetic @openai/codex declares bin/codex.js"
}

function Stop-LoopbackServer {
    if ($null -ne $script:serverProcess -and -not $script:serverProcess.HasExited) {
        Stop-Process -Id $script:serverProcess.Id -Force -ErrorAction SilentlyContinue
        $script:serverProcess.WaitForExit()
    }
    $script:serverProcess = $null
}

function Restart-LoopbackServer {
    Stop-LoopbackServer
    $script:mutationToken = ""
    Start-LoopbackServer
}

function Add-DuplicateEnvironmentDeclaration {
    [Environment]::SetEnvironmentVariable($script:secretName, $script:secretValue, "Process")
    $configPath = Join-Path $appHome "config.json"
    $config = Get-Content -Raw -LiteralPath $configPath | ConvertFrom-Json
    $environmentProperty = $config.PSObject.Properties["environment"]
    $entries = @()
    if ($null -ne $environmentProperty -and $null -ne $environmentProperty.Value) {
        $entries = @($environmentProperty.Value)
    }
    $entries += [pscustomobject]@{ name = $script:secretName; scope = "process"; purpose = "Phase 2 duplicate warning fixture" }
    $entries += [pscustomobject]@{ name = $script:secretName; scope = "process"; purpose = "Phase 2 duplicate warning fixture" }
    if ($null -eq $environmentProperty) {
        $config | Add-Member -NotePropertyName "environment" -NotePropertyValue @()
    }
    $config.environment = $entries
    $config | ConvertTo-Json -Depth 32 | Set-Content -LiteralPath $configPath -Encoding utf8
    Assert-Condition (Test-Path -LiteralPath $configPath -PathType Leaf) "duplicate environment fixture is written to temporary app state"
}

function Assert-NoSecretCanary {
    param([object[]]$Values)

    foreach ($value in $Values) {
        $text = if ($null -eq $value) { "" } else { $value | ConvertTo-Json -Depth 32 -Compress }
        Assert-Condition (-not $text.Contains($script:secretValue)) "structured output does not expose the environment value"
    }
    $serverWasRunning = $null -ne $script:serverProcess -and -not $script:serverProcess.HasExited
    if ($serverWasRunning) {
        Stop-LoopbackServer
    }
    try {
        $found = $false
        if (Test-Path -LiteralPath $appHome) {
            foreach ($file in Get-ChildItem -LiteralPath $appHome -Recurse -Force -File) {
                $bytes = [IO.File]::ReadAllBytes($file.FullName)
                if ([Text.Encoding]::UTF8.GetString($bytes).Contains($script:secretValue)) {
                    $found = $true
                    break
                }
            }
        }
        Assert-Condition (-not $found) "app-home files do not persist the environment value"

        $processFiles = @()
        if (Test-Path -LiteralPath $processOutputRoot -PathType Container) {
            $processFiles = @(Get-ChildItem -LiteralPath $processOutputRoot -Recurse -Force -File | Where-Object { $_.FullName -ne $script:serverLogPath -and $_.FullName -ne $script:serverErrorPath })
        }
        $processFound = $false
        foreach ($file in $processFiles) {
            $content = [Text.Encoding]::UTF8.GetString([IO.File]::ReadAllBytes($file.FullName))
            if ($content.Contains($script:secretValue)) {
                $processFound = $true
                break
            }
        }
        Assert-Condition (-not $processFound) "saved process output does not expose the environment value"

        $serverFound = $false
        foreach ($path in @($script:serverLogPath, $script:serverErrorPath)) {
            if ([string]::IsNullOrWhiteSpace($path) -or -not (Test-Path -LiteralPath $path -PathType Leaf)) {
                continue
            }
            if ([Text.Encoding]::UTF8.GetString([IO.File]::ReadAllBytes($path)).Contains($script:secretValue)) {
                $serverFound = $true
                break
            }
        }
        Assert-Condition (-not $serverFound) "saved server stdout and stderr logs do not expose the environment value"
    }
    finally {
        if ($serverWasRunning) {
            $script:mutationToken = ""
            Start-LoopbackServer
        }
    }
}

function Start-LoopbackServer {
    $script:serverLogPath = Join-Path $processOutputRoot "server.stdout.log"
    $script:serverErrorPath = Join-Path $processOutputRoot "server.stderr.log"
    $serverArgs = @("serve", "--home", $appHome, "--listen", $script:listenAddress)
    $argumentText = (($serverArgs | ForEach-Object { Quote-ProcessArgument ([string]$_) }) -join " ")
    $script:serverProcess = Start-Process -FilePath $script:appBinary -ArgumentList $argumentText `
        -WorkingDirectory $repositoryRoot -RedirectStandardOutput $script:serverLogPath `
        -RedirectStandardError $script:serverErrorPath -WindowStyle Hidden -PassThru

    $deadline = (Get-Date).AddSeconds(30)
    do {
        if ($script:serverProcess.HasExited) {
            $details = if (Test-Path -LiteralPath $script:serverErrorPath) { Get-Content -Raw -LiteralPath $script:serverErrorPath } else { "" }
            throw ("loopback server exited before readiness: " + $details.Trim())
        }
        try {
            $health = Invoke-WebRequest -UseBasicParsing -Uri ("http://$($script:listenAddress)/api/health") -TimeoutSec 2
            if ($health.StatusCode -eq 200) {
                break
            }
        }
        catch {
            # The server may still be binding its loopback socket.
        }
        Start-Sleep -Milliseconds 200
    } while ((Get-Date) -lt $deadline)

    Assert-Condition (-not $script:serverProcess.HasExited) "loopback server stays running"
    $healthData = Invoke-LoopbackJson "/api/health"
    Assert-Condition ([bool]$healthData.ok -and $healthData.network_mode -eq "loopback-only") "health reports loopback-only mode"
}

function Assert-EmptyAssuranceReadPaths {
    $readPaths = @(
        "/api/assurance/sessions",
        "/api/assurance/campaigns",
        "/api/assurance/runs",
        "/api/assurance/invocations",
        "/api/assurance/artifacts",
        "/api/assurance/effects",
        "/api/assurance/pricing"
    )
    foreach ($path in $readPaths) {
        $data = Invoke-LoopbackJson $path
        Assert-Condition (@($data).Count -eq 0) ("Assurance empty read path: " + $path)
    }

    $dashboard = Invoke-LoopbackJson "/api/assurance/dashboard"
    Assert-Condition (@($dashboard.effects).Count -eq 0 -and @($dashboard.invocations).Count -eq 0) "Assurance dashboard starts empty"
    Assert-Condition ([int64]$dashboard.totalTokens -eq 0 -and $dashboard.costState -eq "unknown") "empty dashboard has bounded unknown cost state"
    Assert-Condition ([bool]$dashboard.usageComplete) "empty dashboard reports complete usage coverage"
}

function Assert-UIContract {
    $response = Invoke-WebRequest -UseBasicParsing -Uri ("http://$($script:listenAddress)/") -TimeoutSec 10
    Assert-Condition ($response.StatusCode -eq 200) "UI shell returns HTTP 200"
    $html = [string]$response.Content
    Assert-Condition (-not $html.Contains("__MUTATION_TOKEN__")) "UI shell replaces the mutation token placeholder"
    Assert-Condition ($html -match 'id="main-content"[^>]*tabindex="0"[^>]*aria-labelledby="view-title"') "UI main region has a keyboard focus target and label"
    Assert-Condition ($html -match 'id="home-onboarding"' -and $html -match 'id="assurance-refresh"') "UI shell exposes first-use and Assurance controls"
    Assert-Condition ($html -match 'id="environment"[^>]*aria-live="polite"' -and $html -match 'id="provider-statuses"[^>]*aria-live="polite"') "UI diagnostic regions announce state changes"
}

function Assert-CodexNpmRecovery {
    param([string]$ProjectID)

    $provider = Invoke-AppCliJson @("assurance", "provider") | Where-Object { [string]$_.provider -eq "codex" } | Select-Object -First 1
    Assert-Condition ($null -ne $provider) "Codex provider status is present after npm launcher recovery"
    Assert-Condition ($provider.state -eq "ready" -and $provider.commandFound -eq $true -and $provider.launchTrusted -eq $true -and $provider.profileReady -eq $true) "Codex npm launcher is trusted only after recovery checks"
    $resolved = @($provider.resolvedCommand | ForEach-Object { [string]$_ })
    Assert-Condition ($resolved.Count -eq 2 -and [IO.Path]::GetFileName($resolved[0]) -ieq "node.exe" -and $resolved[1].Replace("\", "/").EndsWith("/node_modules/@openai/codex/bin/codex.js", [StringComparison]::OrdinalIgnoreCase)) "Codex resolves to node.exe plus the declared package bin/codex.js"

    $session = Invoke-AppCliJson @(
        "assurance", "session", "create", "--project", $ProjectID, "--repository", "repo-2",
        "--worktree", "primary", "--provider", "codex", "--model", "phase2-synthetic"
    )
    $sessionID = [string]$session.metadata.id
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($sessionID)) "Codex recovery session is created for the second repository"
    $invocation = Invoke-AppCliJson @(
        "assurance", "invocation", "run", "--session", $sessionID, "--provider", "codex",
        "--profile", "codex", "--model", "phase2-synthetic", "--prompt", "inspect the second repository"
    )
    $syntheticRunMarker = Join-Path $appHome "runtime\codex\output-schema.json.synthetic-run"
    Assert-Condition (Test-Path -LiteralPath $syntheticRunMarker -PathType Leaf) "the synthetic codex.js was reached through node.exe"
    Assert-Condition ([string]$invocation.spec.state -eq "succeeded" -and $invocation.spec.rawTranscript -eq $false) "synthetic Codex npm launcher completes a real local typed invocation"
    Assert-Condition ([string]$invocation.spec.structured.summary -eq "합성 Codex launcher를 node.exe로 실행했습니다.") "synthetic Codex result is reduced to the fixed structured schema"
    Assert-Condition ([int64]$invocation.spec.usage.totalTokens -eq 26) "synthetic Codex invocation records bounded usage"
    Assert-Condition (-not (Test-Path -LiteralPath (Join-Path $appHome "codex.cmd"))) "Codex recovery does not copy or execute the cmd shim inside app state"
    return $invocation
}

function Assert-NoForbiddenLauncherExecution {
    Stop-LoopbackServer
    $files = @()
    if (Test-Path -LiteralPath $processOutputRoot -PathType Container) {
        $files = @(Get-ChildItem -LiteralPath $processOutputRoot -Recurse -Force -File)
    }
    $combined = ($files | ForEach-Object { [Text.Encoding]::UTF8.GetString([IO.File]::ReadAllBytes($_.FullName)) }) -join "`n"
    Assert-Condition (-not $combined.Contains("cmd.exe") -and -not $combined.Contains(".cmd") -and -not $combined.Contains(".bat")) "saved process output and server logs contain no shell launcher execution"
    Assert-Condition (Test-Path -LiteralPath $script:codexLauncherPath -PathType Leaf) "Codex cmd shim remains only an unexecuted discovery anchor"
}

function Assert-DuplicateEnvironmentWarning {
    Add-DuplicateEnvironmentDeclaration
    Restart-LoopbackServer
    $health = Invoke-AppCliJson @("env", "doctor") -AllowFailure
    $status = @($health.environment | Where-Object { [string]$_.name -eq $script:secretName })
    Assert-Condition ($status.Count -eq 1 -and $status[0].state -eq "duplicate") "environment doctor groups duplicate declarations into one warning"
    $finding = @($health.findings | Where-Object { [string]$_.target -eq $script:secretName })
    Assert-Condition ($finding.Count -eq 1 -and [string]$finding[0].type -eq ("environment." + $script:secretName.ToLowerInvariant() + ".duplicate")) "duplicate warning has one remediation finding"
    Assert-NoSecretCanary @($health)
}

function Assert-ProviderRecovery {
    $missing = Invoke-AppCliJson @(
        "agent", "profile", "add", "--id", "phase2-recovery", "--name", "Phase 2 recovery fixture",
        "--command", "phase2-missing-provider", "--version-probe", "version", "--timeout", "3",
        "--launch-mode", "direct", "--data-boundary", "local"
    )
    Assert-Condition ([string]$missing.metadata.id -eq "phase2-recovery") "provider recovery fixture profile is registered"
    Restart-LoopbackServer
    $before = Invoke-AppCliJson @("env", "doctor") -AllowFailure
    $beforeProfile = @($before.profiles | Where-Object { [string]$_.id -eq "phase2-recovery" })
    Assert-Condition ($beforeProfile.Count -eq 1 -and $beforeProfile[0].state -eq "unavailable") "provider recovery starts in an unavailable state"
    Assert-Condition (@($before.findings | Where-Object { [string]$_.target -eq "phase2-recovery" }).Count -eq 1) "unavailable provider profile has one actionable finding"

    $recovered = Invoke-AppCliJson @(
        "agent", "profile", "update", "phase2-recovery", "--name", "Phase 2 recovery fixture",
        "--command", $script:gitPath, "--version-probe", "version", "--timeout", "3",
        "--launch-mode", "direct", "--data-boundary", "local"
    )
    Assert-Condition ([string]$recovered.metadata.id -eq "phase2-recovery") "provider recovery fixture profile is updated"
    Restart-LoopbackServer
    $after = Invoke-AppCliJson @("env", "doctor") -AllowFailure
    $afterProfile = @($after.profiles | Where-Object { [string]$_.id -eq "phase2-recovery" })
    Assert-Condition ($afterProfile.Count -eq 1 -and $afterProfile[0].state -eq "available") "provider profile becomes available after a verified local launcher is configured"
    Assert-Condition (@($after.findings | Where-Object { [string]$_.target -eq "phase2-recovery" }).Count -eq 0) "provider recovery clears the profile finding"
    Assert-NoSecretCanary @($before, $after)
}

try {
    Assert-IsolatedTempRoot
    New-Item -ItemType Directory -Path $runtimeRoot -Force | Out-Null
    New-Item -ItemType Directory -Path $appHome -Force | Out-Null
    New-Item -ItemType Directory -Path $processOutputRoot -Force | Out-Null

    $gitCommand = Get-Command git -CommandType Application -ErrorAction Stop | Select-Object -First 1
    $script:gitPath = $gitCommand.Source
    $goCommand = Get-Command go -CommandType Application -ErrorAction Stop | Select-Object -First 1

    if ([string]::IsNullOrWhiteSpace($BinaryPath)) {
        $script:appBinary = Join-Path $runtimeRoot "dev-control-room-journey.exe"
        $build = Invoke-ProcessChecked -Name "build" -FilePath $goCommand.Source `
            -Arguments @("build", "-trimpath", "-o", $script:appBinary, ".\cmd\dev-control-room")
        Assert-Condition (Test-Path -LiteralPath $script:appBinary) "temporary app binary built outside the repository"
    }
    else {
        $script:appBinary = (Resolve-Path -LiteralPath $BinaryPath).ProviderPath
        Assert-Condition (Test-Path -LiteralPath $script:appBinary -PathType Leaf) "provided app binary exists"
    }

    New-SafeGitFixture
    New-SyntheticCodexNpmLayout
    $script:listenAddress = "127.0.0.1:" + (Get-FreeLoopbackPort $Port)
    Start-LoopbackServer
    Assert-UIContract

    $help = Invoke-AppCliJson @("--help")
    Assert-Condition (@($help.commands) -contains "project" -and [string]$help.example -match "project add") "CLI help exposes the first-use path"

    $initialProjects = Invoke-AppCliJson @("project", "list")
    Assert-Condition (@($initialProjects).Count -eq 0) "fresh app home starts without projects"
    $initialState = Invoke-LoopbackJson "/api/state"
    Assert-Condition (@($initialState.projects).Count -eq 0) "first-use HTTP state has no projects"
    Assert-EmptyAssuranceReadPaths

    $addedProject = Invoke-AppCliJson @("project", "add", "--name", "Phase 2 fixture", "--path", $fixturePath)
    $projectID = [string]$addedProject.metadata.id
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($projectID)) "first-use project registration returns a project id"
    $addedRepository = Invoke-AppCliJson @(
        "project", "repository", "add", "--project", $projectID, "--id", "repo-2",
        "--name", "Phase 2 repository two", "--path", $fixturePathTwo
    )
    Assert-Condition ([string]$addedRepository.metadata.id -eq "repo-2") "first-use flow registers a second repository in the same project"
    $scan = Invoke-AppCliJson @("project", "scan")
    Assert-Condition ([string]$scan.status -eq "completed") "first-use scan produces repository observations"

    $establishedProjects = Invoke-AppCliJson @("project", "list")
    Assert-Condition (@($establishedProjects).Count -eq 1) "return state lists the registered project"
    $establishedState = Invoke-LoopbackJson "/api/state"
    Assert-Condition (@($establishedState.projects).Count -eq 1) "established HTTP state has one project"
    $establishedRepos = @($establishedState.projects[0].repos)
    Assert-Condition ($establishedRepos.Count -eq 2) "established state exposes both registered repositories"
    $firstRepositoryState = @($establishedRepos | Where-Object { [string]$_.id -eq "repo-1" })
    $secondRepositoryState = @($establishedRepos | Where-Object { [string]$_.id -eq "repo-2" })
    Assert-Condition ($firstRepositoryState.Count -eq 1 -and $secondRepositoryState.Count -eq 1) "established state keeps stable identities for both repositories"
    Assert-Condition (@($firstRepositoryState[0].worktrees).Count -ge 1 -and @($secondRepositoryState[0].worktrees).Count -ge 1) "established state exposes a Worktree for each repository"

    $codexInvocation = Assert-CodexNpmRecovery -ProjectID $projectID

    $findingsCLI = Invoke-AppCliJson @("finding", "list", "--project", $projectID, "--repository", "repo-1")
    Assert-Condition (@($findingsCLI).Count -ge 1) "scan produces a reviewable Finding for the local fixture"
    $findingID = [string]$findingsCLI[0].metadata.id
    $findingDetail = Invoke-AppCliJson @("finding", "show", $findingID)
    Assert-Condition ([string]$findingDetail.metadata.id -eq $findingID -and @($findingDetail.spec.evidenceRefs).Count -ge 1) "Finding detail retains a bounded evidence reference"
    $findingsHTTP = Invoke-LoopbackJson ("/api/findings?project_id={0}&repository_id=repo-1" -f $projectID)
    Assert-Condition (@($findingsHTTP).Count -ge 1) "HTTP Finding list supports evidence drill-down"

    $baseline = Invoke-AppCliJson @(
        "assurance", "baseline", "create", "--project", $projectID, "--repository", "repo-1",
        "--worktree", "primary", "--target-branch", "main"
    )
    Assert-Condition ([string]$baseline.spec.state -eq "fresh" -and -not [string]::IsNullOrWhiteSpace([string]$baseline.spec.sourceDigest)) "PR CI baseline stores fresh local evidence"
    Assert-Condition (@($baseline.spec.entries | Where-Object { $_.classification -eq "required" }).Count -ge 1) "baseline retains an explicit required check"
    Assert-Condition (@($baseline.spec.entries | Where-Object { $_.classification -eq "observed" }).Count -ge 1) "baseline retains an observed workflow command"

    $providerCLI = Invoke-AppCliJson @("assurance", "provider")
    $providerHTTP = Invoke-LoopbackJson "/api/assurance/providers"
    foreach ($source in @(@($providerCLI), @($providerHTTP))) {
        $providerNames = @($source | ForEach-Object { [string]$_.provider })
        Assert-Condition ($providerNames.Count -eq (@($providerNames | Sort-Object -Unique).Count)) "provider status response has one row per provider"
        Assert-Condition (@($source | Where-Object { $_.state -in @("ready", "detected", "not_configured", "auth_required", "unavailable") }).Count -eq @($source).Count) "provider states use the grouped status contract"
    }
    Assert-Condition (@($providerCLI | Where-Object { $_.provider -in @("claude", "gemini") -and $_.state -eq "not_configured" }).Count -eq 2) "optional providers remain neutral when unconfigured"

    $session = Invoke-AppCliJson @(
        "assurance", "session", "create", "--project", $projectID, "--repository", "repo-1",
        "--worktree", "primary", "--provider", "fake", "--model", "phase2-fixture"
    )
    $sessionID = [string]$session.metadata.id
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($sessionID)) "Assurance session is resumable after first use"
    $invocation = Invoke-AppCliJson @(
        "assurance", "invocation", "run", "--session", $sessionID, "--provider", "fake",
        "--profile", "fake", "--model", "phase2-fixture", "--scenario", "success"
    )
    $invocationID = [string]$invocation.metadata.id
    Assert-Condition ([string]$invocation.spec.state -eq "succeeded" -and $invocation.spec.rawTranscript -eq $false) "fake Provider leaves a structured, non-transcript result"
    Assert-Condition (@($invocation.spec.artifactIds).Count -ge 1 -and [int64]$invocation.spec.usage.totalTokens -gt 0) "Provider result retains bounded usage and an artifact reference"
    $invocationDetail = Invoke-AppCliJson @("assurance", "invocation", "show", "--id", $invocationID)
    Assert-Condition ([string]$invocationDetail.metadata.id -eq $invocationID) "invocation evidence is reviewable by id"

    $campaign = Invoke-AppCliJson @(
        "assurance", "campaign", "create", "--project", $projectID, "--repository", "repo-1",
        "--worktree", "primary", "--name", "Phase 2 quality fixture", "--session", $sessionID
    )
    $campaignID = [string]$campaign.metadata.id
    $qualityRun = Invoke-AppCliJson @(
        "assurance", "run", "--campaign", $campaignID, "--technique", "static_security",
        "--provider", "fake", "--model", "phase2-fixture"
    )
    Assert-Condition ([string]$qualityRun.spec.state -eq "succeeded" -and $qualityRun.spec.evidence.result.executed -eq $true) "static Quality Run executes the registered local runner"
    Assert-Condition ([string]$qualityRun.spec.runner -eq "quality.go.vet" -and @($qualityRun.spec.artifactIds).Count -ge 1) "Quality Run records its runner and artifact evidence"
    $blockedQuality = Invoke-AppCliExpectedFailure @(
        "assurance", "run", "--campaign", $campaignID, "--technique", "mutation"
    ) "unavailable|사용할 수 없습니다"
    Assert-Condition ($blockedQuality.ExitCode -ne 0) "mutation Quality Run reports that the native mutation tool is unavailable in this offline verifier"

    $effect = Invoke-LoopbackMutationJson -Method "POST" -Path "/api/assurance/effects" -Body @{
        projectId = $projectID
        repositoryId = "repo-1"
        worktreeId = "primary"
        fingerprint = "sha256:phase2-journey-effect"
        kind = "measured"
        sourceRunId = [string]$qualityRun.metadata.id
        evidenceIds = @($qualityRun.spec.artifactIds)
        adopted = $false
        reverified = $true
        label = "local Quality Run evidence"
        value = 1
        unit = "verified-run"
    }
    Assert-Condition ([string]$effect.spec.kind -eq "measured" -and [string]$effect.spec.sourceRunId -eq [string]$qualityRun.metadata.id) "dashboard effect record links to the Quality Run"

    $actionHead = [string]$firstRepositoryState[0].worktrees[0].spec.head
    $actionPlan = Invoke-AppCliJson @(
        "action", "plan", "--id", "phase2-approval", "--name", "Phase 2 approval fixture",
        "--project", $projectID, "--repository", "repo-1", "--worktree", "primary",
        "--type", "release.production", "--input", ("commit=" + $actionHead)
    )
    Assert-Condition ($actionPlan.spec.approvalRequired -eq $true -and $actionPlan.spec.policyDecision -eq "approval_required") "high-impact Action exposes an approval gate"
    Invoke-AppCliExpectedFailure @("action", "execute", "phase2-approval") "approval|admit|승인" | Out-Null

    $establishedDashboard = Invoke-AppCliJson @("assurance", "dashboard")
    Assert-Condition (@($establishedDashboard.effects).Count -ge 1 -and @($establishedDashboard.invocations).Count -ge 1) "Assurance dashboard shows evidence and Provider value"
    Assert-Condition ([int64]$establishedDashboard.totalTokens -gt 0 -and [bool]$establishedDashboard.usageComplete) "dashboard shows complete bounded usage"
    $establishedRead = Invoke-LoopbackJson "/api/assurance/dashboard"
    Assert-Condition (@($establishedRead.effects).Count -ge 1 -and @($establishedRead.invocations).Count -ge 1) "established HTTP Assurance dashboard remains populated"
    $runsRead = Invoke-LoopbackJson "/api/assurance/runs"
    $artifactsRead = Invoke-LoopbackJson "/api/assurance/artifacts"
    Assert-Condition (@($runsRead | Where-Object { $_.spec.state -eq "succeeded" }).Count -ge 1) "HTTP Quality Run read path exposes the successful result"
    Assert-Condition (@($artifactsRead).Count -ge 2) "HTTP artifact read path exposes Provider and Quality evidence"

    Assert-ProviderRecovery
    Assert-DuplicateEnvironmentWarning

    Restart-LoopbackServer
    $returnProjects = Invoke-AppCliJson @("project", "list")
    $returnDashboard = Invoke-AppCliJson @("assurance", "dashboard")
    Assert-Condition (@($returnProjects).Count -eq 1 -and @($returnDashboard.effects).Count -ge 1 -and @($returnDashboard.invocations).Count -ge 1) "returning user sees persisted project and Assurance value after restart"
    $returnState = Invoke-LoopbackJson "/api/state"
    $returnRepos = @($returnState.projects[0].repos)
    Assert-Condition (@($returnState.projects).Count -eq 1 -and $returnRepos.Count -eq 2) "returning HTTP state restores both repositories"
    Assert-Condition (@($returnRepos | Where-Object { @($_.worktrees).Count -ge 1 }).Count -eq 2) "returning HTTP state restores a Worktree for each repository"
    Assert-UIContract
    Assert-NoSecretCanary @($help, $providerCLI, $providerHTTP, $findingDetail, $baseline, $codexInvocation, $invocation, $qualityRun, $effect, $actionPlan, $returnDashboard)
    Assert-NoForbiddenLauncherExecution

    Write-Host ("PASS  Phase 2 journey verification completed with {0} assertions" -f $script:passCount)
    Write-Host ("TEMP  " + $runtimeRoot)
    Write-Host "LIMIT  The Codex check launches only the synthetic local npm fixture; no real provider/company endpoints, screenshots, or full keyboard traversal are exercised."
}
finally {
    Stop-LoopbackServer
    [Environment]::SetEnvironmentVariable($script:secretName, $script:previousSecretValue, "Process")
    [Environment]::SetEnvironmentVariable("Path", $script:previousPath, "Process")
    if ($KeepTemp) {
        Write-Host ("Kept temporary journey root: " + $runtimeRoot)
    }
    elseif (Test-Path -LiteralPath $runtimeRoot) {
        Assert-IsolatedTempRoot
        Remove-Item -LiteralPath $runtimeRoot -Recurse -Force -ErrorAction SilentlyContinue
        Write-Host ("CLEANUP  removed only the isolated temporary journey root: " + $runtimeRoot)
    }
}
