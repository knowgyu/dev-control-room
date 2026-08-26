# v0.10.2 verification record

Status: candidate and final full gates passed; final package and release
assets contain the authoritative records.
Date: 2026-08-27
Candidate feature source commit: `d0d2d333c5de0613d1fc3071e558828e557596d2`

## Scope

This patch verifies the established Home effect-proof summary and its link to
the existing Assurance data. It keeps verified, trace-complete, measured, and
estimated results distinct and displays `확인 불가` when the data has no
supporting record. It does not promote native process resilience, provider
authority, mutation, human patch adoption, causal attribution, or full manual
accessibility to complete.

## Required final gate

Run from a clean native Windows checkout:

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.10.2-final
pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.10.2 -OutputDirectory artifacts\0.10.2
pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1
```

The generated `verification-summary.json` is authoritative for the exact
release source commit, tool versions, command results, and cross-build hashes.
`artifacts/0.10.2/SHA256SUMS` is authoritative for portable ZIP hashes. The
published release assets are the final user-facing copy of both records.

## Candidate full gate

`scripts/verify.ps1 -Mode Full` passed on the feature source commit above.
Environment: Windows PowerShell 7.6.4, Go 1.26.7 windows/amd64, Node
v24.15.0, Git 2.53.0.windows.1, and gcc 16.2.0 (MSYS2).

```text
gofmt check                          PASS
go test -count=1 ./...              PASS (124.011s)
node --check internal/app/ui/app.js PASS
go test -count=1 -race ./...        PASS (152.630s)
go vet ./...                        PASS
go mod verify                       PASS
go build ./...                      PASS
Windows amd64 cross-build           PASS
Windows arm64 cross-build           PASS
git diff --check                    PASS
```

Candidate cross-build hashes:

```text
3276958BA24EDF702308EC3DE1A319E73C77EF1C367AED66D9A756D85C0735A8  dev-control-room-windows-amd64.exe
292D2C324A7D3DC47A53943024251B522534203D0ABA0E17DFB799F576DCE5B2  dev-control-room-windows-arm64.exe
```

## Focused and journey evidence

```text
node --check internal/app/ui/app.js                                  PASS
go test -count=1 ./internal/app -run 'TestEmbeddedUI(ExposesKoreanMultiViewControlRoom|AssuranceDashboardContract|FirstUseAndFindingTargetContract|KeyboardRouteFocusContract|DialogFocusAndDescriptionContract)' PASS
pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1           PASS (239 assertions)
```

The fresh candidate local-server browser observation is recorded in
[USER_JOURNEY_ACCEPTANCE.md](USER_JOURNEY_ACCEPTANCE.md). It verified the
populated Home effect-proof summary, its detailed Assurance link, desktop and
narrow layout, no horizontal overflow at the narrow viewport, and no browser
console errors.

## Not run by automation

- Explorer double-click observation on the final v0.10.2 ZIP.
- Full Tab/Shift+Tab/Enter/Space traversal, native dialog Esc delivery, and
  assistive-technology acceptance; the in-app Browser driver did not deliver
  those key sequences reliably.
- A second clean physical Windows device.
- Company GitHub/Jenkins/Kubernetes endpoints, credentials, production, or
  destructive cleanup.

No not-run item is treated as a pass. See
[NATIVE_WINDOWS_SMOKE.md](NATIVE_WINDOWS_SMOKE.md),
[VERIFICATION_PLAYBOOK.md](VERIFICATION_PLAYBOOK.md), and
[HANDOFF.md](HANDOFF.md) for the remaining acceptance boundary.
