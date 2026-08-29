# v0.11.1 verification record

Status: native full gate, native resilience acceptance, Phase 2 journey,
amd64 package smoke, and local browser smoke passed.
Date: 2026-08-30

## Scope

This patch release reports binary version `0.11.1` and limits published
portable artifacts to Windows amd64. The v0.11.0 native process-boundary and
assurance retry behavior is carried into this release without changing the
local-only, fail-closed execution boundaries.

## Focused evidence

The following checks passed on native Windows PowerShell 7.6 with Go 1.26.7
windows/amd64:

- `pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full`
- `pwsh -NoProfile -File .\scripts\verify-native-resilience.ps1` — PASS
  (15 assertions)
- `pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1` — PASS
  (239 assertions)
- `pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.11.1` — one
  Windows amd64 ZIP and `SHA256SUMS`; no ARM64 release ZIP
- Local packaged-binary browser smoke on `http://127.0.0.1:38471`: all six
  routes loaded, project registration form opened/closed, diagnostics
  recheck completed, assurance Provider filter changed/reset, and browser
  warning/error console entries remained at zero.

The Full gate still builds a Windows arm64 verification binary so source
compatibility is checked. That binary is not included in the release package.
Linux is used by the repository CI test job and is not a release target.

## Required final gate

Run from the clean native Windows checkout:

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.11.1-final
pwsh -NoProfile -File .\scripts\verify-native-resilience.ps1
pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1
pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.11.1 -OutputDirectory artifacts\0.11.1
```

The generated `verification-summary.json` is authoritative for the final
source commit, tool versions, and command results. `artifacts\0.11.1\SHA256SUMS`
is authoritative for the portable ZIP hash.

## Not run by this release

- Real external Provider authentication or approval-prompt acceptance.
- Company GitHub/Jenkins/Kubernetes endpoints or credentials.
- Production or destructive cleanup operations.
- Full Tab/Shift+Tab/Enter/Space traversal, native dialog Esc delivery,
  assistive-technology acceptance, or a second physical Windows device.

No not-run item is treated as a pass.
