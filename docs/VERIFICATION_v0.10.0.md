# v0.10.0 verification record

Status: release-candidate record; the final source commit, generated verifier
summary, Windows package hashes, and release URL are recorded at publication.
Date: 2026-08-27

## Scope

This candidate verifies the B restart-boundary slice and the F
effect-evidence hardening slice. It does not promote native process
resilience, provider authority, mutation, human patch adoption, causal
attribution, or manual accessibility to complete.

## Required final gate

Run from a clean native Windows checkout:

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.10.0-final
pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.10.0 -OutputDirectory artifacts\0.10.0
```

The generated `verification-summary.json` is authoritative for the exact
source commit, tool versions, command results, and cross-build hashes.
`artifacts/0.10.0/SHA256SUMS` is authoritative for the portable ZIP hashes.
The published release assets are the final user-facing copy of both records.

## Focused evidence already completed

```text
go test -count=1 ./internal/app -run 'TestAssuranceImpact|TestEffectVerification|TestEmbeddedUIAssuranceDashboardContract' -v  PASS
go test -count=1 ./cmd/dev-control-room -run 'Test.*Help|Test.*Doctor|Test.*Assurance' -v  PASS
go test -count=1 ./...  PASS
node --check internal/app/ui/app.js  PASS
gofmt -l <changed Go files>  PASS
git diff --check  PASS (line-ending warnings only)
```

## Not run by automation

- Explorer double-click observation on the final ZIP.
- Full Tab/Shift+Tab/Enter/Space traversal, native dialog Esc delivery, and
  assistive-technology acceptance.
- A second clean physical Windows device.
- Company GitHub/Jenkins/Kubernetes endpoints, credentials, production, or
  destructive cleanup.

No not-run item is treated as a pass. See
[NATIVE_WINDOWS_SMOKE.md](NATIVE_WINDOWS_SMOKE.md),
[VERIFICATION_PLAYBOOK.md](VERIFICATION_PLAYBOOK.md), and
[HANDOFF.md](HANDOFF.md) for the remaining acceptance boundary.
