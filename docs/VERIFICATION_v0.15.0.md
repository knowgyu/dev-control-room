# v0.15.0 verification record

Status: release candidate; final source gate and publication are pending.
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

The following focused checks passed on the working tree before the release
candidate commit:

- `node --check internal/app/ui/app.js`
- `go test -count=1 ./internal/app -run 'TestEmbeddedUI'`
- `go test -count=1 ./...`
- `git diff --check`
- Working-tree browser smoke: all five guide slides render concrete action
  fields, slide deep links survive reload, route links navigate to the expected
  view, and the desktop guide has no horizontal overflow.

## Final source gates

The exact candidate commit, full verifier output, native resilience assertions,
and Phase 2 journey count will be recorded here after the release gates run.

## Publication

The package name, SHA-256, GitHub release URL, CI result, downloaded-archive
verification, and extracted-binary browser smoke will be recorded here after
publication. Expected release assets are:

```text
dev-control-room_0.15.0_windows_amd64.zip
SHA256SUMS
```
