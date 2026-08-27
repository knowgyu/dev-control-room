# v0.11.0 verification record

Status: focused native resilience acceptance passed; final full gate and
publication assets are recorded by the generated summary/log
Date: 2026-08-28
Implementation source commit: `943119e`

## Scope

This slice makes the ProcessRunner stdin and process-tree boundary explicit.
Provider and diagnostic commands receive EOF instead of an inherited console.
On Windows, timeout and cancellation use the Job Object attached to the
process to terminate child processes as well. The service still fails closed
at a restart boundary and never blindly resumes an old Provider PID.

## Focused evidence

The following checks passed on native Windows PowerShell 7.6.4 with Go 1.26.7
windows/amd64:

- `go test -count=1 ./internal/environment -run 'TestProcessRunner(ClosedStdinReturnsEOF|TimeoutKillsProcessTree|CancellationKillsProcessTree|TimeoutCancellationAndBoundedStreams)$' -v`
- `go test -count=1 ./internal/app -run 'TestStartupRecoveryMarksActiveInvocationInterruptedWithoutRelaunch|TestRetryInterruptedInvocationCreatesIdempotentChildWithoutPromptPersistence' -v`
- `pwsh -NoProfile -File .\scripts\verify-native-resilience.ps1` — PASS (15 assertions)

The native ProcessRunner fixture observed immediate stdin EOF, passed the
existing output-boundary/timeout test, terminated a spawned child tree after
timeout, and terminated a spawned child tree after context cancellation. The
application fixture rechecked restart-boundary recovery and the idempotent
user-directed retry child attempt without launching a second Provider.

## Required final gate

Run from the clean native Windows checkout:

`pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.11.0-final`

`pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1`

`pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.11.0 -OutputDirectory artifacts\0.11.0`

The generated `verification-summary.json` is authoritative for the final
source commit, tool versions, command results, and cross-build hashes.
`artifacts\0.11.0\SHA256SUMS` is authoritative for portable ZIP hashes.

## Not run by this slice

- Automatic discovery or resumption of an old Provider process. This remains
  intentionally unsupported and fail-closed.
- Expired authentication or an unexpected approval prompt against a real
  external Provider.
- Company GitHub/Jenkins/Kubernetes endpoints, credentials, production, or
  destructive operations.
- Full Tab/Shift+Tab/Enter/Space traversal, native dialog Esc delivery,
  assistive-technology acceptance, or a second physical Windows device.

No not-run item is treated as a pass. See `MILESTONE_B_VERIFICATION.md`,
`NATIVE_WINDOWS_SMOKE.md`, and `HANDOFF.md` for the acceptance boundary.
