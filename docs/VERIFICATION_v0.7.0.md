# v0.7.0 verification record

Candidate source commit: `8bcf1a7915d1e4997494ac7b6758ac51a7c794d7`.
Full-run time: 2026-08-26 09:21:08–09:24:19 KST.

## Environment

- Windows PowerShell 7.6.4
- Go 1.26.7 windows/amd64
- Node v24.15.0
- Git 2.53.0.windows.1
- gcc 16.2.0 (MSYS2)

## Automated checks

- `gofmt -l`: PASS.
- `go test -count=1 ./...`: PASS (80.498s).
- `node --check internal/app/ui/app.js`: PASS.
- `go test -count=1 -race ./...`: PASS (100.945s).
- `go vet ./...`: PASS.
- `go mod verify`: PASS.
- `go build ./...`: PASS.
- Windows amd64 and arm64 builds: PASS.
- `git diff --check`: PASS.
- `pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1`: PASS,
  65 assertions. It built an isolated executable, app home, and local Git
  fixture, then verified first-use, return, loopback health/state, Provider
  grouping, and empty Assurance API/dashboard read paths.

The Full-run summary and log are published with this release as
`verification-summary.json` and `verification.log`.

## Windows package check

- `pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.7.0
  -OutputDirectory artifacts\0.7.0`: PASS.
- Both ZIP hashes matched the generated `SHA256SUMS`.
- The amd64 ZIP contains the executable, license/policy, native smoke/playbook,
  and the v0.7.0 release notes and verification record.
- The unpacked amd64 executable returned `version --json` = `0.7.0` and served
  `/api/health` in loopback-only mode from a fresh package-local home.

Final release assets are rebuilt from the tagged record commit. Their ZIP
hashes are authoritative only in that release's `SHA256SUMS` asset.

## Cross-build hashes

These are the binary hashes from the exact candidate verification run. ZIP
hashes are authoritative in the release `SHA256SUMS` asset.

```text
fe216ecf40acbb1a594ee66a7009819bcdd2ad68d3137e70f342fcd24d59899a  dev-control-room-windows-amd64.exe
709c3ce3f4c1dbc8dd05b158be664a1667693eb50da9486d55fef40f627dceaa  dev-control-room-windows-arm64.exe
```

## Local Windows UI observation

- A fresh local home served the exact amd64 candidate at loopback. The
  `검증` route displayed the Assurance dashboard, its repeatability/evidence/
  cost/no-transcript cards, Provider/model filters, metrics, and empty states;
  Browser console errors were empty.
- The same dashboard rendered at a 390 px viewport with filters and metrics
  visible.
- Route navigation moved focus to `#main-content` with `:focus-visible`.
  `Enter` on `새로 고침` was accepted and the control returned enabled.

## Not run or not claimed

- Full Tab traversal did not move reliably after initial `main` focus in the
  in-app Browser, so it remains unaccepted.
- A real Codex task invocation/authentication, non-TTY/closed-stdin acceptance,
  and Windows process-tree cancellation were not run.
- Provider-authoritative GitHub/Jenkins evidence, company endpoints,
  credentials, proxy, production, and a second physical Windows device were
  not contacted.
- The current Quality Run reports are fixture-backed; this record does not
  claim an installed mutation, property, fuzz, static/security, or targeted
  E2E tool executed.
