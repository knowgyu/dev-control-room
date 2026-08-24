# Slice F verification: typed Action execution

Status: implemented with WSL evidence on 2026-08-24. Native Windows 11 /
PowerShell 7.6 acceptance remains a separate operator gate.

## Delivered contract

- `ActionRun` is a versioned, exact Project/Repository/Worktree result with
  bounded stdout/stderr, exit code, precheck/postcheck evidence, status, and
  immutable persistence.
- The Broker is the only process execution owner. It consumes an admitted
  persisted plan and cannot accept an executable, arguments, shell text,
  working directory, environment values, or approval from an adapter.
- Execution rechecks the plan digest, admission scope, lease, approval, exact
  Worktree context, and explicit execution trust immediately before launch.
- The application service refreshes read-only Git Worktree evidence before and
  after execution. A changed or unverifiable target fails closed.
- Child processes use typed argv, the server-owned environment allowlist,
  explicit Worktree directory, timeout, process-tree containment, bounded
  output, and masking before persistence or presentation.
- CLI, loopback HTTP, and embedded UI expose plan listing, Worktree
  execution-ready transition, execution, approval ceremony, and result review
  through the shared application service.

## WSL verification

The following passed with the temporary Go 1.26.7 Linux toolchain while the
module remains `go 1.23.0`:

```text
gofmt -w <touched Go files>
go test -count=1 ./...
go test -count=1 -race ./...
go vet ./...
CGO_ENABLED=0 go build ./cmd/dev-control-room
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/dev-control-room
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/dev-control-room
go mod verify
git diff --check
```

Focused regression coverage includes successful typed execution, exact
Worktree fail-closed behavior, nonzero exit handling, masked output, atomic
run/event persistence, UI-only Worktree trust, and loopback execute routing.

## Native Windows gate

Run later from the native NTFS checkout in PowerShell 7.6. Capture the complete
append-only output and record the exact commit:

```powershell
go test ./...
CGO_ENABLED=1 go test -race ./...
go vet ./...
go build ./...
go mod verify
```

Prepare the isolated local-only fixture with the checked-in script. It uses
dedicated `$acceptanceRoot` and `$appDataRoot` variables, passes the app data
root explicitly, and never assigns PowerShell's reserved home variable or
deletes any existing user data:

```powershell
.\scripts\prepare-slice-f-acceptance.ps1 -BinaryPath .\artifacts\dev-control-room.exe
```

The script creates a temporary bare Git origin and Worktree, registers them in
a fresh application data directory, runs Guidance/handoff/cleanup/MCP read-only
smokes, starts the fixture server on `127.0.0.1:38472`, and prints a JSON context
containing the exact paths, IDs, source commit, and server PID. Preserve that
temporary root until its evidence has been reviewed.

Also manually verify, using a generic fixture only:

- UI trust transition and native approval ceremony remain separate;
- admitted `repository.refresh` runs in the selected Worktree directory;
- argv is preserved without shell interpretation;
- timeout/cancellation kills the full child process tree;
- stdout/stderr and SQLite/API/CLI results contain no secret canary;
- changed HEAD, moved/unverified Worktree, expired approval, lock conflict,
  and nonzero exit all prevent a false success;
- a second request with the same idempotency key does not start a second
  process.

This WSL verification does not claim native Windows runtime behavior. Scheduler
install/uninstall, company release procedures, destructive cleanup, and
external connectors remain out of scope for this slice.
