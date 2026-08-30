# v0.15.0 verification record

Status: PASS for the final source candidate; publication is pending.
Date: 2026-08-31

## Scope

This release packages the concrete first-use guide and its documentation updates.
The guide connects the actual route `프로젝트 → 진단 → 개선 → 작업 → 검증 → 활동`,
shows a click sequence and completion criterion for each step, and keeps the
existing deep link contract `#guide?slide=N`.

No approval, Action Broker, artifact-integrity, provider, or MCP execution
boundary is widened by this change. The release package remains Windows amd64
only; arm64 and Linux are not release targets.

## Pre-gate checks

The following focused checks passed before the release candidate commit:

- `node --check internal/app/ui/app.js`
- `go test -count=1 ./internal/app -run 'TestEmbeddedUI'`
- `go test -count=1 ./...`
- `git diff --check`
- Working-tree browser smoke: all five guide slides render concrete action
  fields, slide deep links survive reload, route links navigate to the expected
  view, and the desktop guide has no horizontal overflow.

## Final source gates

The final source candidate is commit
`c70074c13c486b68f62c451665e9f73fb1a03402` (`docs: ship concrete usage guide
in v0.15.0`). The native Windows checkout passed:

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.15.0-final
pwsh -NoProfile -File .\scripts\verify-native-resilience.ps1
pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1
```

The Full gate passed formatting, `go test -count=1 ./...`, JavaScript syntax,
`go test -count=1 -race ./...`, `go vet ./...`, module verification, normal
builds, Windows amd64 and arm64 cross-builds, and `git diff --check`. The
generated summary is `artifacts/verification-v0.15.0-final/verification-summary.json`;
it records Go 1.26.7, Node 24.15.0, PowerShell 7.6.4, and a 255.939-second
regular test run plus a 301.934-second race test run.

The native Windows resilience gate passed 15 assertions covering closed stdin,
timeout/cancellation process-tree cleanup, restart recovery, and bounded retry
behavior. The Phase 2 local journey gate passed 373 assertions covering first
use, loopback HTTP, repository registration, provider recovery and invocation,
quality evidence, mutation availability boundaries, approval blocking,
persistence after restart, secret masking, and the embedded UI contracts.

## Publication

The package name, SHA-256, GitHub release URL, CI result, downloaded-archive
verification, and extracted-binary browser smoke will be recorded here after
publication. Expected release assets are:

```text
dev-control-room_0.15.0_windows_amd64.zip
SHA256SUMS
```
