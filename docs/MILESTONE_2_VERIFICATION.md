# Milestone 2 verification

Milestone 2 adds read-only environment/profile diagnostics and a typed,
fixture-only Windows scheduling boundary. Milestone 3 execution features and
later integrations were not started.

## Implemented scope

- Agent Profiles are versioned domain objects with a command/executable,
  `direct` or `powershell_profile` launch mode, version-probe argument array,
  bounded timeout, data boundary, and environment-name allowlist. The initial
  templates are `codex`, `claude`, `gemini`, and `claude-local`; no model name,
  organization name, repository name, endpoint, or credential is a default.
  Explicit direct paths and the PowerShell host must be local Windows `.exe`
  files; externally resolved profile commands must be on a local drive. UNC
  paths, traversal segments, alternate data streams, and direct shell-wrapper
  targets are rejected.
- Profiles are stored in the existing SQLite `agent_profiles` table and survive
  restart. Migration 3 adds only `environment_health` and `scheduler_state`;
  migrations 1 and 2 were not edited. Local config version 3 preserves the
  additive Milestone 2 metadata when migrating version 2 and records one-time
  default Profile initialization, so removing every Profile survives restart.
- Environment Doctor resolves the six baseline tools and configured profiles.
  Direct profiles use executable resolution and argument arrays. Profile-backed
  commands use a fixed PowerShell metadata probe and retain only command type,
  resolved path, and parsed version metadata. Version probes accept one reviewed
  version argument only, run from an allowed temporary directory, and parse the
  final version candidate to avoid host/profile banner misclassification.
- Environment declarations retain names, scopes, and consumer metadata only.
  Health results contain declared/missing/duplicate/conflict states. Connector
  results contain only reference presence and the configured last validation
  result; failed/unavailable prior validation makes Health unavailable. Secret
  values are never loaded into a returned object.
- `devroom env doctor`, `devroom env status`, and
  `devroom agent profile list|show|add|update|remove` use the existing
  `devroom/cli/v1` envelope and exit-code contract. HTTP `GET /api/environment`
  is cached and side-effect free; explicit, token-protected
  `POST /api/environment/doctor` performs refresh. `/api/agent-profiles` uses
  the same application service. The UI shows Environment Health, severity,
  unavailable reasons, next actions, and a protected manual Doctor button.
- The scheduler package defines typed install, uninstall, status, and dry-run
  operations. It validates the application-owned task name, absolute Windows
  executable path, daily catch-up, and ignore-new duplicate policy. UNC paths,
  traversal segments, alternate data streams, and non-`.exe` launch targets are
  rejected. The only
  adapter in this milestone is a fake adapter; no native Task Scheduler
  registration or deletion is performed.
- Child process environments are explicitly constructed from a small runtime
  allowlist plus a profile's configured names. Stdout and stderr have separate
  bounds, timeout/cancellation is enforced, and Unix process groups plus a
  Windows Job Object with kill-on-close are used for process-tree cancellation.
  An explicitly empty child environment remains empty rather than falling back
  to parent inheritance. No generic shell or arbitrary Task Scheduler
  PowerShell surface was added.

## Acceptance follow-up

The full `main..codex/milestone-2` diff was reviewed against the acceptance
boundaries. The follow-up changes harden PowerShell command-name validation,
make scheduler arguments an exact typed `serve --home <Windows path>` shape,
route scheduler status through the fake typed adapter, reject non-Windows
paths in the Windows-targeted doctor, mask PowerShell probe output before
metadata parsing, and add process-tree cancellation coverage. No native
Task Scheduler adapter was introduced.

The final review additionally fixed empty-allowlist parent environment
inheritance, Health remaining available for missing/conflicting declarations,
unauthenticated HTTP GET refresh, discarded PowerShell version output,
re-seeding Profiles after the user removed all of them, unversioned config
growth, scheduler path edge cases, and inherited UNC working-directory version
misclassification. It also corrected the version JSON contract to report
Milestone 2 instead of the prior milestone.

## Checks executed in WSL

Using a temporary Go 1.27.0 toolchain while retaining the module's Go 1.23
target:

```text
gofmt -w <all touched Go files>
go test -count=1 -race ./...
go vet ./...
go mod verify
CGO_ENABLED=0 go build -o <temporary Linux output> ./cmd/dev-control-room
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o <temporary Windows amd64 output> ./cmd/dev-control-room
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -o <temporary Windows arm64 output> ./cmd/dev-control-room
```

The tests cover direct and PowerShell-profile resolution, missing and failed
probes, version parsing, timeout/cancellation, bounded output, environment
duplicate/conflict/missing findings, profile CRUD restart persistence, SQLite
and CLI/HTTP secret-canary absence (including a scan of every SQLite table
cell), CLI envelopes, migration 3, and fake Task Scheduler
install/uninstall/status/dry-run, catch-up, duplicate-instance, typed
argument validation, and WSL-path rejection.
Additional final-review tests cover empty child environments, config v2-to-v3
migration, persistent removal of all default Profiles, read-only HTTP status,
protected HTTP Doctor refresh, connector failure health, scheduler path edge
cases, and unrelated host-version rejection.

## Native Windows and intentionally unexecuted operations

- A native Windows Go toolchain was not available in this WSL session, so
  native `go test`, `go vet`, and native compilation remain pending.
- PowerShell 7.6.5 native read-only smoke passed using the Windows amd64
  cross-build and a fresh temporary Windows home. It passed version JSON,
  `powershell_profile` Agent Profile CRUD and actual `1.2.3` version parsing
  through a temporary synthetic command, `env status --json`,
  `env doctor --json`, invalid WSL and relative Windows path rejection,
  scheduler dry-run and app-owned fake status, loopback `/api/health`, a
  concurrent read-only CLI query while `serve` was running, and absence of
  `secret-canary-value` from stdout, stderr, HTTP/CLI JSON, and temporary home
  files (including SQLite).
  The expected unavailable-capability exit code was 6 for both environment
  queries; their JSON envelopes were valid. Windows interop `cmd.exe` launched
  the native binary directly from WSL to avoid inheriting the WSL-hosted
  PowerShell Job boundary; Environment Doctor itself launched and inspected
  `pwsh.exe 7.6.5`. The synthetic command, binaries, output, SQLite database,
  and temporary Windows directory were removed after verification; no user
  PowerShell profile was modified.
- No native Windows Task Scheduler task was installed or uninstalled, and no
  native scheduler API was queried. The smoke's status operation used only the
  app-owned exact task name through the fake typed adapter.
- No external network call, telemetry, automatic update, connector
  authentication, Checkset execution, Action broker execution, release,
  Jenkins, Kubernetes, Harbor, MCP, or AI workflow was added or run.

## Security boundary

Secret values are not fields in the profile, declaration, connector, health,
finding, event, SQLite singleton, CLI, HTTP, or UI contracts. Masking remains
before persistence and presentation. Doctor probes reduce bounded, masked
command output to non-secret version metadata, and connector checks retain only
reference presence and non-secret prior validation status. Fixture tests use
temporary directories and synthetic names; no user project path or credential
was inspected.
