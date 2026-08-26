# v0.10.0 verification record

Status: full candidate gate passed; final release summary and package hashes
are attached to the publication record.
Date: 2026-08-27
Candidate source commit: `d4d0070e5b065e739653d5cc1eaffb666067d023`

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
release source commit, tool versions, command results, and cross-build hashes.
`artifacts/0.10.0/SHA256SUMS` is authoritative for the portable ZIP hashes.
The published release assets are the final user-facing copy of both records.

## Full candidate gate

`scripts/verify.ps1 -Mode Full` passed on the candidate source commit above.
Environment: Windows PowerShell 7.6.4, Go 1.26.7 windows/amd64, Node
v24.15.0, Git 2.53.0.windows.1, and gcc 16.2.0 (MSYS2).

```text
gofmt check                          PASS
go test -count=1 ./...              PASS (123.541s)
node --check internal/app/ui/app.js PASS
go test -count=1 -race ./...        PASS (149.031s)
go vet ./...                        PASS
go mod verify                       PASS
go build ./...                      PASS
Windows amd64 cross-build           PASS
Windows arm64 cross-build           PASS
git diff --check                    PASS
```

Candidate cross-build hashes:

```text
138118C3029152CE3352CB0F41A0F4B1620A44EA091BE7268AF8D342815198D9  dev-control-room-windows-amd64.exe
B1314CAE7438DA42D6D231050651344C69CF1215BDF6E472A24E2925F1854DCC  dev-control-room-windows-arm64.exe
```

The final release-commit verifier summary is uploaded with the release when
the documentation-only release commit is made. Its source and hashes take
precedence over this candidate snapshot.

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
