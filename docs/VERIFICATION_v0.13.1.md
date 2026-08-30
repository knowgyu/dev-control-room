# v0.13.1 verification record

Status: PASS for the local candidate and downloaded GitHub release.
Date: 2026-08-30

## Scope

This patch release carries the v0.13 product-surface work and fixes the
diagnostics layout defect where a proportional provider-state column created
misleading empty space on wide displays. It also verifies the released Windows
amd64 archive after download.

## Source gates

Run from the native Windows checkout:

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.13.1-final
pwsh -NoProfile -File .\scripts\verify-native-resilience.ps1
pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1
pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.13.1 -OutputDirectory artifacts\0.13.1
```

The focused UI and syntax checks are:

```powershell
go test ./internal/app -run 'TestEmbeddedUI(ExposesKoreanMultiViewControlRoom|InformationArchitectureContract|ProviderCapabilityGroupingContract|FirstUseAndFindingTargetContract)$' -count=1
go test ./cmd/dev-control-room ./internal/mcp ./internal/folderpicker -count=1
node --check .\internal\app\ui\app.js
```

The full verifier passed `go test`, race-enabled tests, `go vet`, module
verification, the normal build, Windows amd64 and arm64 cross-builds, and
`git diff --check`. The native resilience gate passed 15 assertions, and the
Phase 2 journey gate passed 353 assertions. The arm64 binary is verification
only; the portable package contains Windows amd64 only.

The candidate package is:

```text
artifacts\0.13.1-candidate\dev-control-room_0.13.1_windows_amd64.zip
SHA-256: authoritative in artifacts\0.13.1-candidate\SHA256SUMS
```

## Browser acceptance

- the diagnostics route has one visible view and one page heading;
- provider state, provider name, and recovery action remain aligned without a
  proportional empty status column;
- the required-tool health row stays compact;
- the browser reports no visible horizontal overflow or console errors;
- the downloaded release serves the same embedded asset versions and reports
  `0.13.1` from both CLI and MCP metadata.

No company Jenkins, production release, or destructive cleanup is part of this
verification. Jenkins MCP execution remains behind the existing Action Broker
approval boundary.

## Remote release acceptance

The GitHub release is [v0.13.1](https://github.com/knowgyu/dev-control-room/releases/tag/v0.13.1).
Its Windows amd64 archive was downloaded into
`artifacts\0.13.1-downloaded` and checked against the published `SHA256SUMS`:

```text
068ef373bc31a9d3c856aa9ba457a72db280a4fc471b13054468838252991ff0  dev-control-room_0.13.1_windows_amd64.zip
```

The extracted binary reported version `0.13.1` and returned a successful JSON
help envelope. The running server at `127.0.0.1:38471` is the extracted release
binary, and the browser loaded `/ui/app.css?v=0.13.1` and
`/ui/app.js?v=0.13.1` on the `진단` route. The visible route had one heading,
no document-level horizontal overflow, and no captured browser error or
warning logs.
