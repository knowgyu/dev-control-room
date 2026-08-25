# v0.6.0 verification record

Source commit for the full run: `535657d34f45a8bac9a8f7031b8c368732734eab`.
Run time: 2026-08-26 02:09:29–02:13:31 KST.

## Automated checks

- Focused Phase 1 A–G tests: pass.
- Focused Phase 2 provider, CLI, HTTP, and UI checks: pass.
- `go test -count=1 ./...`: PASS (76.253s).
- `CGO_ENABLED=1 go test -count=1 -race ./...`: PASS (97.875s).
- `go vet ./...`: PASS.
- `go mod verify`: PASS.
- `go build ./...`: PASS.
- `node --check internal/app/ui/app.js`: pass.
- Windows amd64 and arm64 cross-builds: PASS.
- `git diff --check`: PASS.

The final PowerShell summary is `artifacts/verification-0.6.0/verification-summary.json`.

## Cross-build hashes

These are the binary hashes from the full verification run. The release ZIP
hashes are published separately in `SHA256SUMS`.

```text
2018c886b0955a680f19cdcc8ce45f5edafda4a2fdad865db9c024f6cf1d597c  dev-control-room-windows-amd64.exe
bf9501adab122d0e9142121869fda98fa0e4f3a14426d2859fea741f42d1181b  dev-control-room-windows-arm64.exe
```

Release ZIP hashes are authoritative in the packaged `SHA256SUMS` file.

## Local Windows journey

- PowerShell server and CLI ran with Windows paths.
- First-use, established-home, grouped Provider, narrow viewport, and primary keyboard activation were checked in the in-app Browser.
- Screenshots are documented in `docs/USER_JOURNEY_ACCEPTANCE.md`.

## Not run

- Real Codex task invocation and provider authentication.
- Company Jenkins, GitHub, Kubernetes, proxy, or production endpoint checks.
- Manual acceptance on a second physical Windows 11 device.
- Full keyboard traversal in the in-app Browser could not be observed reliably after the initial main focus; this remains a documented gap.
