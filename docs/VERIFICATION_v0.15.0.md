# v0.15.0 verification record

Status: PASS; source gates and publication smoke are complete.
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

## Publication update

The [v0.15.0 GitHub release](https://github.com/knowgyu/dev-control-room/releases/tag/v0.15.0)
was published from the verified source line with exactly two assets:

```text
dev-control-room_0.15.0_windows_amd64.zip
SHA256SUMS
```

The published ZIP SHA-256 is:

```text
2b1959b115aa6d7b4690fd5c31c192d35b0c3097abf46870c6c5c507cea3b2f5  dev-control-room_0.15.0_windows_amd64.zip
```

The ZIP was downloaded again from the GitHub release, matched the published
checksum, extracted successfully, and its executable returned version `0.15.0`.
The archive contains no Linux or arm64 release asset.

The downloaded executable was served loopback-only on native Windows at
`http://127.0.0.1:38476`. Browser smoke verified all seven routes have one
visible heading, no horizontal overflow, and the version-pinned `v=0.15.0`
CSS/JS assets. The Usage route kept `04 / 05` after loading
`#guide?slide=4`, and the next control advanced it to `05 / 05` with the
expected concrete verification step.

The GitHub Checks run
[33323527232](https://github.com/knowgyu/dev-control-room/actions/runs/33323527232)
passed both jobs: native Windows behavioral/race tests and Linux formatting,
module/vet checks, and Windows amd64 cross-build. GitHub emitted only the
existing Node 20 deprecation and Linux cache-restore annotations; neither job
failed.
