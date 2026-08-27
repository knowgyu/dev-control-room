# v0.10.3 verification record

Status: final gate pending
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

To be filled from the final clean-source run. No final release claim is made
until the full gate, journey, package smoke, and hash checks pass.

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
