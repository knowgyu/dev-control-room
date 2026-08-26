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
$fixturePath = Join-Path $runtimeRoot "git-fixture"
$processOutputRoot = Join-Path $runtimeRoot "process-output"
$serverProcess = $null
$serverLogPath = ""
$serverErrorPath = ""
$script:processCount = 0
$script:passCount = 0
$script:gitPath = ""
$script:appBinary = ""
$script:listenAddress = ""
$script:serverProcess = $null

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
    param([string[]]$Arguments)

    $cliArguments = @($Arguments)
    $isHelp = $cliArguments.Count -gt 0 -and $cliArguments[0] -in @("--help", "-h", "help")
    if (-not $isHelp) {
        $cliArguments += @("--home", $appHome)
    }
    $cliArguments += "--json"
    $result = Invoke-ProcessChecked -Name ("cli-" + ($Arguments[0] -replace '[^a-zA-Z0-9_-]', '_')) `
        -FilePath $script:appBinary -Arguments $cliArguments
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

function New-SafeGitFixture {
    New-Item -ItemType Directory -Path $fixturePath -Force | Out-Null
    Invoke-GitChecked @("init", "--quiet", $fixturePath)
    Invoke-GitChecked @("-C", $fixturePath, "config", "user.name", "Phase 2 Journey Fixture")
    Invoke-GitChecked @("-C", $fixturePath, "config", "user.email", "phase2-journey@example.invalid")
    Invoke-GitChecked @("-C", $fixturePath, "branch", "-M", "main")
    Set-Content -LiteralPath (Join-Path $fixturePath "README.md") -Value "# Phase 2 journey fixture`n" -Encoding utf8
    Invoke-GitChecked @("-C", $fixturePath, "add", "--", "README.md")
    Invoke-GitChecked @("-C", $fixturePath, "commit", "--quiet", "-m", "initial fixture commit")
    $status = @(& $script:gitPath -C $fixturePath status --short 2>&1)
    $statusCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
    Assert-Condition ($statusCode -eq 0 -and $status.Count -eq 0) "temporary Git fixture is clean"
}

function Start-LoopbackServer {
    $serverLogPath = Join-Path $processOutputRoot "server.stdout.log"
    $serverErrorPath = Join-Path $processOutputRoot "server.stderr.log"
    $serverArgs = @("serve", "--home", $appHome, "--listen", $script:listenAddress)
    $argumentText = (($serverArgs | ForEach-Object { Quote-ProcessArgument ([string]$_) }) -join " ")
    $script:serverProcess = Start-Process -FilePath $script:appBinary -ArgumentList $argumentText `
        -WorkingDirectory $repositoryRoot -RedirectStandardOutput $serverLogPath `
        -RedirectStandardError $serverErrorPath -WindowStyle Hidden -PassThru

    $deadline = (Get-Date).AddSeconds(30)
    do {
        if ($script:serverProcess.HasExited) {
            $details = if (Test-Path -LiteralPath $serverErrorPath) { Get-Content -Raw -LiteralPath $serverErrorPath } else { "" }
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

try {
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
    $script:listenAddress = "127.0.0.1:" + (Get-FreeLoopbackPort $Port)
    Start-LoopbackServer

    $help = Invoke-AppCliJson @("--help")
    Assert-Condition (@($help.first_use) -contains "project add") "CLI help exposes the first-use path"

    $initialProjects = Invoke-AppCliJson @("project", "list")
    Assert-Condition (@($initialProjects).Count -eq 0) "fresh app home starts without projects"
    $initialState = Invoke-LoopbackJson "/api/state"
    Assert-Condition (@($initialState.projects).Count -eq 0) "first-use HTTP state has no projects"
    Assert-EmptyAssuranceReadPaths

    $addedProject = Invoke-AppCliJson @("project", "add", "--name", "Phase 2 fixture", "--path", $fixturePath)
    $projectID = [string]$addedProject.metadata.id
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($projectID)) "first-use project registration returns a project id"

    $establishedProjects = Invoke-AppCliJson @("project", "list")
    Assert-Condition (@($establishedProjects).Count -eq 1) "return state lists the registered project"
    $establishedState = Invoke-LoopbackJson "/api/state"
    Assert-Condition (@($establishedState.projects).Count -eq 1) "established HTTP state has one project"
    Assert-Condition (@($establishedState.projects[0].repos).Count -ge 1) "established state exposes the registered repository"

    $providerCLI = Invoke-AppCliJson @("assurance", "provider")
    $providerHTTP = Invoke-LoopbackJson "/api/assurance/providers"
    foreach ($source in @(@($providerCLI), @($providerHTTP))) {
        $providerNames = @($source | ForEach-Object { [string]$_.provider })
        Assert-Condition ($providerNames.Count -eq (@($providerNames | Sort-Object -Unique).Count)) "provider status response has one row per provider"
        Assert-Condition (@($source | Where-Object { $_.state -in @("ready", "detected", "not_configured", "auth_required", "unavailable") }).Count -eq @($source).Count) "provider states use the grouped status contract"
    }

    $establishedDashboard = Invoke-AppCliJson @("assurance", "dashboard")
    Assert-Condition (@($establishedDashboard.effects).Count -eq 0 -and @($establishedDashboard.invocations).Count -eq 0) "return state keeps Assurance empty until a run exists"
    $establishedRead = Invoke-LoopbackJson "/api/assurance/dashboard"
    Assert-Condition (@($establishedRead.effects).Count -eq 0 -and @($establishedRead.invocations).Count -eq 0) "established HTTP Assurance dashboard remains readable"

    Write-Host ("PASS  Phase 2 journey verification completed with {0} assertions" -f $script:passCount)
    Write-Host ("TEMP  " + $runtimeRoot)
}
finally {
    if ($null -ne $script:serverProcess -and -not $script:serverProcess.HasExited) {
        Stop-Process -Id $script:serverProcess.Id -Force -ErrorAction SilentlyContinue
        $script:serverProcess.WaitForExit()
    }
    if ($KeepTemp) {
        Write-Host ("Kept temporary journey root: " + $runtimeRoot)
    }
    elseif (Test-Path -LiteralPath $runtimeRoot) {
        Remove-Item -LiteralPath $runtimeRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
