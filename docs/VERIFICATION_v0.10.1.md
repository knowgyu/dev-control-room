# v0.10.1 verification record

Status: candidate full gate passed; final release summary and package hashes
are attached to the publication record.
Date: 2026-08-27
Candidate source commit: `37e8c1055230d7c5ebe8b3905bbd8469d4de15c0`

## Scope

This patch verifies the first-use focus correction, inline registration focus
restoration, and empty Assurance trend reduction. It does not promote native
process resilience, provider authority, mutation, human patch adoption, causal
attribution, or full manual accessibility to complete.

## Required final gate

Run from a clean native Windows checkout:

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.10.1-final
pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.10.1 -OutputDirectory artifacts\0.10.1
pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1
```

The generated `verification-summary.json` is authoritative for the exact
release source commit, tool versions, command results, and cross-build hashes.
`artifacts/0.10.1/SHA256SUMS` is authoritative for portable ZIP hashes. The
published release assets are the final user-facing copy of both records.

## Candidate full gate

`scripts/verify.ps1 -Mode Full` passed on the candidate source commit above.
Environment: Windows PowerShell 7.6.4, Go 1.26.7 windows/amd64, Node
v24.15.0, Git 2.53.0.windows.1, and gcc 16.2.0 (MSYS2).

```text
gofmt check                          PASS
go test -count=1 ./...              PASS (123.875s)
node --check internal/app/ui/app.js PASS
go test -count=1 -race ./...        PASS (148.413s)
go vet ./...                        PASS
go mod verify                       PASS
go build ./...                      PASS
Windows amd64 cross-build           PASS
Windows arm64 cross-build           PASS
git diff --check                    PASS
```

Candidate cross-build hashes:

```text
8A79FD1CA648572EE88CBC20EEA6E110133C20DE5DC7749A3792BC6C8FDFB8DD  dev-control-room-windows-amd64.exe
1C8C9620AA2C2D9D58D762975AB349E843C9E3E50BAB4AAE3C32692FE670519A  dev-control-room-windows-arm64.exe
```

## Focused and journey evidence

```text
node --check internal/app/ui/app.js                                  PASS
go test -count=1 ./internal/app -run 'TestEmbeddedUI(KeyboardRouteFocusContract|DialogFocusAndDescriptionContract|AssuranceDashboardContract)' PASS
pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1           PASS (239 assertions)
```

The fresh packaged-source browser observation is recorded in
[USER_JOURNEY_ACCEPTANCE.md](USER_JOURNEY_ACCEPTANCE.md). It verified the
initial body focus, route focus target, registration opener restoration, empty
trend state, desktop/narrow screenshots, and no browser console errors.

## Not run by automation

- Explorer double-click observation on the v0.10.1 ZIP.
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
