# v0.10.3 verification record

Status: candidate full gate and package smoke passed; final publication uses
the clean documentation commit and attached generated summary
Date: 2026-08-27
Candidate feature source commit: `dcba07d`

## Scope

This patch adds an explicit user-directed retry for a persisted
`interrupted` Agent invocation. The retry is a new child attempt, not an
automatic restart or an old-process resume. It requires a new bounded
one-line prompt, does not persist prompt text, links parent and child IDs, and
uses deterministic idempotency so a repeated request does not launch a second
Provider process.

## Focused evidence

The following checks passed before the release gate:

- `go test -count=1 ./internal/app -run 'TestRetryInterruptedInvocationCreatesIdempotentChildWithoutPromptPersistence|TestStartupRecoveryMarksActiveInvocationInterruptedWithoutRelaunch|TestEmbeddedUIAssuranceInvocationRetryRouteRequiresMutationToken|TestEmbeddedUIAssuranceDashboardContract' -v`
- `go test -count=1 ./cmd/dev-control-room -run 'TestNestedCLIHelpIncludesUsageAndRequiredArguments|TestAssuranceLifecycleCLIUsesNamedFlagsAndStableEnvelopes' -v`
- `node --check internal/app/ui/app.js`
- `git diff --check`

The focused retry test proves the child/parent relationship, prompt
non-persistence, idempotent repeat, conflicting-prompt rejection, succeeded
child non-retryability, and newline rejection. The UI contract test proves
retry markup and refresh behavior are embedded, while the route test proves
the mutation-token and loopback Origin boundary.

## Required final gate

Run from the clean native Windows checkout:

`pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.10.3-final`

`pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.10.3 -OutputDirectory artifacts\0.10.3`

`pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1`

The generated `verification-summary.json` is authoritative for the exact
release source commit, tool versions, command results, and cross-build hashes.
`artifacts/0.10.3/SHA256SUMS` is authoritative for portable ZIP hashes.

## Final gate result

The candidate full gate passed on source commit `ade184c` in Windows PowerShell
7.6.4 with Go 1.26.7 windows/amd64, Node v24.15.0, Git 2.53.0.windows.1, and
gcc 16.2.0 (MSYS2):

- gofmt check — PASS
- `go test -count=1 ./...` — PASS (173.210s)
- `node --check internal/app/ui/app.js` — PASS
- `go test -count=1 -race ./...` — PASS (243.440s)
- `go vet ./...` — PASS
- `go mod verify` — PASS
- `go build ./...` — PASS
- Windows amd64 cross-build — PASS
- Windows arm64 cross-build — PASS
- `git diff --check` — PASS

Candidate cross-build hashes were:

- `069051F706B0423661913D751C0001FF3C90E79A14E643D41D3E36FB41F29FE2`
  `dev-control-room-windows-amd64.exe`
- `FF53CF9EC199F9DB1195910AB3FDF032D1936904BB066CCF015AFAEC4636F294`
  `dev-control-room-windows-arm64.exe`

The Phase 2 clean-state journey passed 239 assertions. The v0.10.3 package
was created for amd64 and arm64; both ZIPs matched the generated `SHA256SUMS`.
The extracted amd64 package passed `version --json`, `--help --json`,
`troubleshoot --home`, loopback-only `/api/health`, and UI shell smoke.

The final clean-source gate is rerun after this documentation commit. Its
`artifacts/verification-v0.10.3-final/verification-summary.json`, log, final
cross-build hashes, and the final package `SHA256SUMS` are the publication
source of truth.

## Not run by automation

- Explorer double-click observation on the final v0.10.3 ZIP.
- A browser session with a seeded `interrupted` invocation and actual retry
  form submission; the available clean-state journey has no durable interrupted
  fixture.
- Full Tab/Shift+Tab/Enter/Space traversal, native dialog Esc delivery, and
  assistive-technology acceptance.
- A second clean physical Windows device.
- Company GitHub/Jenkins/Kubernetes endpoints, credentials, production, or
  destructive cleanup.

No not-run item is treated as a pass. See `NATIVE_WINDOWS_SMOKE.md`,
`VERIFICATION_PLAYBOOK.md`, and `HANDOFF.md` for the remaining acceptance
boundary.
