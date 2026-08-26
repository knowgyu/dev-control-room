# v0.9.1 verification record

Status: candidate verification in progress
Date: 2026-08-27
Environment: current Windows workspace; native interactive acceptance is
recorded separately when it is actually run.

## Automated verification completed for the candidate

| Command | Result | Evidence |
| --- | --- | --- |
| `gofmt -w` on changed Go files | PASS | Changed Go sources and tests formatted. |
| `go test -count=1 ./internal/store -run 'Test(OpenConcurrentInitializationIsSerialized|OpenReturnsContextDeadlineWhenStorageLockWaitExpires|StorageLockReturnsTypedBusyAfterBoundedWait|MigrateDirectCallHonorsStorageLock|StorageServerAndCLIProcessesSerialize|Migration(FourteenAcceptsReleasedMigrationThirteenChecksum|RejectsUnknownMigrationThirteenChecksum))' -v` | PASS | Same-home initialization, timeout/busy distinction, direct migration lock, process serialization, and legacy checksum guard. |
| `go test -count=1 ./internal/store ./cmd/dev-control-room` | PASS | Storage and CLI startup/troubleshooting regression coverage. |
| `go test -count=1 ./...` | PASS | Full Go test suite on the current candidate before release-record finalization. |
| `go test -count=1 -race ./...` | PASS | Full race suite on the current candidate before release-record finalization. |
| `node --check internal/app/ui/app.js` | PASS | Embedded UI JavaScript parses. |
| `git diff --check` | PASS | No whitespace errors. |

## Product checks

- Human CLI errors include `원인`, `조치`, and `다음 명령`.
- `--json` preserves the existing envelope and does not mix human text into
  stderr.
- A failed startup writes only a safe classification to
  `troubleshooting/latest.json`; `troubleshoot` reads readiness without
  displaying the local path or raw database error.
- Direct migration and cross-process storage tests use a temporary database;
  no company endpoint, credential, or production target is used.

## Native and release checks

- Windows amd64/arm64 cross-build: pending final candidate run.
- `go vet ./...`, `go mod verify`, and `go build ./...`: pending final candidate
  run.
- Extracted ZIP, hash, version probe, loopback health, and double-click
  startup: pending final candidate package run.
- Full Tab/Space traversal, native dialog Esc, assistive technology, and a
  second physical Windows device: not run here unless explicitly observed.

No unrun native or external check is treated as a pass.
