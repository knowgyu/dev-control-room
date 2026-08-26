# v0.9.1 verification record

Status: candidate and local package verified; remote publication pending
Date: 2026-08-27
Candidate source commit: `37366fbff02d9369a1e844d76c0440cb1ea66d23`
(`fix: improve startup diagnostics and storage concurrency`)
Environment: Windows PowerShell 7.6.4, Go 1.26.7 windows/amd64, Node
v24.15.0, Git 2.53.0.windows.1, and gcc 16.2.0 (MSYS2).

## Automated verification completed for the candidate

| Command | Result | Evidence |
| --- | --- | --- |
| `gofmt -w` on changed Go files | PASS | Changed Go sources and tests formatted. |
| `go test -count=1 ./internal/store -run 'Test(OpenConcurrentInitializationIsSerialized|OpenReturnsContextDeadlineWhenStorageLockWaitExpires|StorageLockReturnsTypedBusyAfterBoundedWait|MigrateDirectCallHonorsStorageLock|StorageServerAndCLIProcessesSerialize|Migration(FourteenAcceptsReleasedMigrationThirteenChecksum|RejectsUnknownMigrationThirteenChecksum))' -v` | PASS | Same-home initialization, timeout/busy distinction, direct migration lock, process serialization, and legacy checksum guard. |
| `go test -count=1 ./internal/store ./cmd/dev-control-room` | PASS | Storage and CLI startup/troubleshooting regression coverage. |
| `go test -count=1 ./...` | PASS | Full Go test suite on source candidate `37366fb`. |
| `go test -count=1 -race ./...` | PASS | Full race suite on source candidate `37366fb`. |
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

- Windows amd64/arm64 cross-build: PASS, `scripts/verify.ps1 -Mode Full`.
- `go vet ./...`, `go mod verify`, and `go build ./...`: PASS,
  `scripts/verify.ps1 -Mode Full`.
- Extracted ZIP, SHA-256, version probe, loopback health, and native process
  startup guidance: PASS using the amd64 package.
- Explorer double-click observation: not run by automation; the native process
  launch and no-terminal failure output are covered, but an operator should
  still observe the exact Desktop double-click once.
- Full Tab/Space traversal, native dialog Esc, assistive technology, and a
  second physical Windows device: not run here unless explicitly observed.

No unrun native or external check is treated as a pass.

## Cross-build hashes

These hashes are from the full verification run. ZIP hashes remain authoritative
in the release `SHA256SUMS` asset.

```text
D3CDBCD19AD3ED7EC7264A93557F9FF09C9FC6880A74D1153CEB1681CD80A68F  dev-control-room-windows-amd64.exe
E3FB5855FA7BEFFAC406AF45436389A9B952446242BA4A4EE8AA852D0DA0E7F2  dev-control-room-windows-arm64.exe
```

## Package smoke

The package script created both portable ZIPs. Their hashes were checked
against the generated `SHA256SUMS`; the amd64 archive was extracted into a
fresh temporary directory and reported version `0.9.1`, successful help, and a
loopback-only health response. A deliberately invalid local database produced
the Korean `원인`/`조치`/`다음 명령` guidance and a safe troubleshooting record.

The generated `artifacts/0.9.1/SHA256SUMS` is the package hash record. The
published release asset is authoritative because changing the included
verification document necessarily changes the ZIP hash.
