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

## Publication update

The [v0.14.0 GitHub release](https://github.com/knowgyu/dev-control-room/releases/tag/v0.14.0)
was published with exactly two assets:

```text
dev-control-room_0.14.0_windows_amd64.zip
SHA256SUMS
```

The remote ZIP was downloaded again and matched its published SHA-256:

```text
71883db2b7f887f808f24347a58bb48bafcb343fdc1f11b918855a7085aa324d
```

Its extracted executable returned version `0.14.0`, served loopback-only on
native Windows, and passed a browser smoke: all seven routes have one visible
heading, no horizontal overflow or console errors, and the read-only Assurance
demo renders with the version-pinned assets.

After publication, commit `7caadccf075e6ed2b03fc6b8cb67687b7c491b93`
made the CI boundary explicit: behavioral and race tests run on native Windows,
while Linux verifies formatting and a Windows amd64 cross-build. It does not
change the tagged source or release archive. Both jobs passed in
[checks run 33311855078](https://github.com/knowgyu/dev-control-room/actions/runs/33311855078).
