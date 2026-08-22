# Milestone 0 verification

Milestone 0 establishes the contracts and safety foundation only. It does not
add production actions, connectors, MCP, AI workflows, or the Milestone 1
Project/persistence/Doctor feature set.

## Assumptions and unavailable inputs

- The target remains native Windows 11 with PowerShell 7.6. The implementation
  was developed from WSL. A cross-built Windows amd64 executable was run through
  PowerShell 7.6.5 on the Windows host; native compilation still requires a
  Windows Go installation.
- No company repository paths, connector endpoints, credentials, or agent
  profiles were available or observed. Milestone 0 does not need them.
- The old v1 `workbenches` configuration is treated as migration input only;
  the new domain and HTTP surface use Project terminology.

## WSL checks completed

Using Go 1.27.0 temporarily because Go was not installed in the WSL image:

```text
go test -count=1 -race ./...
go vet ./...
go mod verify
go list -m all
go build -o /tmp/dev-control-room-linux ./cmd/dev-control-room
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/dev-control-room-windows-amd64.exe ./cmd/dev-control-room
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -o /tmp/dev-control-room-windows-arm64.exe ./cmd/dev-control-room
```

All commands passed. The Windows outputs were identified as PE32+ amd64 and
PE32+ arm64 executables. The CLI JSON success and invalid-input exit paths and
the loopback `/api/health` response were also exercised from WSL. Domain tests
also verify that only a human can approve protected work, the single human may
approve their own request, any ActionPlan mutation invalidates its approval
digest, and Finding timestamps remain ordered.

## Windows runtime smoke completed

The cross-built Windows amd64 executable was launched as a native Windows
process from PowerShell 7.6.5. Both `version --json` and the loopback
`/api/health` endpoint passed:

```json
{"PowerShell":"7.6.5","Version":"0.1.0-milestone-0","Health":true,"Network":"loopback-only"}
```

## PowerShell 7.6 smoke test for a Windows host

Run from the repository root after installing Go 1.23 or newer:

```powershell
$ErrorActionPreference = 'Stop'
$env:CGO_ENABLED = '0'
go test ./...
go vet ./...
go build -o .\devroom.exe .\cmd\dev-control-room

.\devroom.exe version --json | ConvertFrom-Json | Format-List

$testHome = Join-Path $env:TEMP 'DevControlRoom-M0-Smoke'
Remove-Item -LiteralPath $testHome -Recurse -Force -ErrorAction SilentlyContinue
$process = Start-Process -FilePath .\devroom.exe -ArgumentList @('serve', '--home', $testHome, '--listen', '127.0.0.1:38471') -PassThru
try {
    Start-Sleep -Milliseconds 500
    $health = Invoke-RestMethod -Uri 'http://127.0.0.1:38471/api/health'
    if ($health.schema -ne 'devroom/cli/v1' -or -not $health.ok -or $health.data.network_mode -ne 'loopback-only') {
        throw 'unexpected health envelope'
    }
    $health | ConvertTo-Json -Depth 5
}
finally {
    Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
    Wait-Process -Id $process.Id -Timeout 5 -ErrorAction SilentlyContinue
}
```

The equivalent native Windows `go test`, `go vet`, and compilation commands are
still pending because Go is not installed on the Windows host. Runtime startup,
JSON output, local data-directory creation, and loopback HTTP behavior have been
verified natively with the cross-built executable.
