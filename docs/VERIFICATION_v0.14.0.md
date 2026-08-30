# v0.14.0 verification record

Status: PASS for the local release candidate; remote publication pending.
Date: 2026-08-30

## Candidate

The source candidate is commit `6543ca67139f2e86545a338b960116b4f5dc32d8`
(`feat: add quality improvement loop`). It adds the local, quality-first
improvement workflow while keeping the existing approval and artifact
boundaries fail-closed.

## Source gates

Run from the native Windows checkout:

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.14.0-final
pwsh -NoProfile -File .\scripts\verify-native-resilience.ps1
pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1
```

The full verifier passed formatting, `go test ./...`, JavaScript syntax,
`go test -race ./...`, `go vet ./...`, module verification, normal builds,
Windows amd64 and arm64 cross-builds, and `git diff --check`. Its generated
summary identifies the candidate commit above and records Go 1.26.7, Node
24.15.0, and PowerShell 7.6.4.

The native Windows resilience gate passed 15 assertions covering closed stdin,
timeout and cancellation process-tree cleanup, restart recovery, and bounded
retry behavior. The Phase 2 local journey gate passed 373 assertions: first
use, loopback HTTP, repository registration, evidence flow, bounded provider
invocation, approval blocking, persistence after restart, secret masking, and
the embedded UI contracts.

The arm64 build is verification-only. The release package will contain only a
Windows amd64 ZIP and its `SHA256SUMS`; no Linux or arm64 release asset is
created. Package, extracted-binary browser smoke, and remote-download results
are recorded in the post-publication handoff update.
