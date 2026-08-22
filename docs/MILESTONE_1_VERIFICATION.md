# Milestone 1 verification

Milestone 1 is implemented from the Milestone 0 baseline. Milestone 2 and later
work are intentionally not included.

## Implemented scope

- SQLite is the source of truth for Project, Repository, Observation, Finding,
  Event, ScanRun, and FailureFingerprint objects. The M0 JSONL ledger and
  in-memory snapshot adapters are no longer used.
- `state.db` is created below the configured local application directory and
  uses forward-only schema version 2. Migration 1 was not edited; migration 2
  adds query indexes. Foreign keys are enabled and Repository identity is the
  composite `(project_id, id)` key. WAL mode and a five-second busy timeout
  allow the background service and short-lived CLI processes to share the
  local database without immediate lock failures.
- Project and multi-Repository create/read/update/delete operations share one
  application service across CLI, HTTP, and the embedded UI. Project export and
  import use a versioned, non-secret object and require registered directories.
- Git collection uses explicit `git` arguments, a registered absolute working
  directory, an 8-second timeout, and a 1 MiB output bound. No shell or generic
  executable runner exists. Registered paths are canonicalized and must be the
  actual Git worktree root, so registering a nested directory cannot cause the
  collector to inspect a broader parent repository. Remote URLs are normalized
  without credentials and provider capability is detected locally.
- Dirty state, upstream drift, detached HEAD, missing remote, stale scan, and
  unsafe cleanup are deterministic Findings with stable fingerprints and
  evidence references. Manual and scheduled scans call the same scan path.
  Every scan retains a distinct Observation; incomplete collection produces a
  completed, failed ScanRun and an unavailable-capability result instead of a
  false success.
- M0 Project migration is resumable after an interrupted multi-Project import,
  and aggregate Project saves reconcile repositories omitted from the object.
- `devroom project`, `devroom finding`, `devroom event`, and `devroom serve`
  are implemented. Query commands support the stable `devroom/cli/v1` JSON
  envelope and stable exit codes. The UI centers Project cards, Finding
  severity, and evidence references.
- Existing ActionPlan/Approval validation was preserved, including complete
  plan-digest binding and high-impact approval requirements. No connector,
  Environment Doctor, Task Scheduler, Checkset execution, Action broker
  execution, Agent Handoff, or MCP work was added.

## Checks executed

The checks below were run in WSL with a temporary Go 1.27 toolchain. The module
continues to target Go 1.23:

```text
go version
go version go1.27.0 linux/amd64

gofmt -w <all touched Go files>
go test ./...
PASS (cmd, app, collector, contract, domain, masking, reconcile, store)

go test -count=1 -race ./...
PASS

go vet ./...
PASS

go mod verify
all modules verified

CGO_ENABLED=0 go build -o /tmp/dev-control-room-linux ./cmd/dev-control-room
PASS; ELF x86-64 Linux executable

GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/dev-control-room-windows-amd64.exe ./cmd/dev-control-room
PASS; PE32+ amd64 executable

GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -o /tmp/dev-control-room-windows-arm64.exe ./cmd/dev-control-room
PASS; PE32+ arm64 executable
```

The test suite includes:

- CLI JSON golden and envelope schema validation;
- migration idempotence, schema-version history rejection, composite FK
  isolation, typed repository persistence, aggregate repository reconciliation,
  WAL/busy-timeout configuration, and restart persistence;
- multi-Repository Project CRUD and removal-event durability;
- temporary Git repositories for normalized remotes, provider detection,
  dirty state, detached HEAD, unsafe cleanup, and nested-path boundary rejection;
- unregistered-path non-access using a recording runner;
- distinct Observation history across equivalent scans and failed ScanRun
  finalization after collector errors;
- resumable M0-to-M1 Project migration after a simulated partial import;
- secret-canary checks against SQLite object JSON and returned events;
- manual versus scheduled finding identity parity;
- stream masking split-boundary and UTF-8 boundary coverage;
- high-impact ActionPlan/Approval and internal-error output-hiding checks.

No company repository, credential, connector endpoint, or unregistered path was
used by tests. Git fixtures are created below `t.TempDir()`.

## Native Windows smoke

- PowerShell 7.6.5 launched the Windows amd64 executable with `Start-Process`.
- `version --json` returned the stable success envelope with exit code 0.
- `serve` created a fresh Windows-local data directory, opened SQLite in WAL
  mode, bound `127.0.0.1`, and returned a valid `/api/health` envelope.
- The smoke process and temporary data directory were stopped and removed.

Native Windows `go test`, `go vet`, and compilation were not run because a
Windows Go toolchain is not installed. Both requested Windows architectures
were cross-compiled successfully from the temporary Go toolchain.

## Remaining acceptance

- No real project onboarding was attempted; it requires user-provided
  registered paths and is outside fixture-only verification.

Dependency inventory was re-audited in `THIRD_PARTY_POLICY.md`. No dependency
was added in Milestone 1; the reviewed indirect module graph is unchanged.
