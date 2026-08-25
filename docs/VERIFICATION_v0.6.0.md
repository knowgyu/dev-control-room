# v0.6.0 verification record

Source commit for the full run: `ea31e766b9aaa58cf96c95b06017a2deec535f83`.
Run time: 2026-08-26 02:16:35–02:19:38 KST.

## Automated checks

- Focused Phase 1 A–G tests: pass.
- Focused Phase 2 provider, CLI, HTTP, and UI checks: pass.
- `go test -count=1 ./...`: PASS (77.492s).
- `CGO_ENABLED=1 go test -count=1 -race ./...`: PASS (95.734s).
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
858bd61b348cd8e11b9ff0b18bf409810b7f697e5eab41cc4d5a1193d2b2d845  dev-control-room-windows-amd64.exe
5aa5c529d90049621b14c22e82fd615042b36904b4e3e9d320aee9dddaf9c21a  dev-control-room-windows-arm64.exe
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
