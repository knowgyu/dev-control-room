# v0.13.0 verification record

Status: PASS for the local release candidate.
Date: 2026-08-30

## Scope

This release improves first-use orientation, adds the in-app usage guide,
replaces the legacy Windows folder dialog with the Explorer-backed native
folder picker, and exposes only the approved Jenkins MCP boundary.

## Source gates

Run from the native Windows checkout:

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.13.0-final
pwsh -NoProfile -File .\scripts\verify-native-resilience.ps1
pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1
pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.13.0 -OutputDirectory artifacts\0.13.0
```

The focused pre-release checks are:

```powershell
go test ./internal/mcp ./internal/folderpicker ./internal/app -count=1
go test ./cmd/dev-control-room ./internal/mcp ./internal/folderpicker -count=1
node --check .\internal\app\ui\app.js
```

The final full verifier passed `go test`, race-enabled tests, `go vet`, module
verification, the normal build, Windows amd64 and arm64 cross-builds, and
`git diff --check`. The native resilience gate passed 15 assertions. The
Phase 2 journey gate passed 353 assertions. The arm64 binary is verification
only; the portable package contains Windows amd64 only.

The candidate package is:

```text
artifacts\0.13.0-candidate\dev-control-room_0.13.0_windows_amd64.zip
SHA-256: 7a848d56db8b3d01a79a762da06729f494f5d6477e72c3bc8b789870cafc99ad
```

## Browser acceptance

- all seven hash routes show exactly one visible view and one page heading;
- the guide moves between slides, records `#guide?slide=N`, and restores the
  selected slide after reload;
- the first-use screen does not repeat its main sentence in the onboarding strip;
- desktop and narrow layouts have no visible horizontal overflow;
- the populated project inventory keeps its project name on a readable line
  instead of collapsing it into one-character wrapping;
- diagnostic provider rows keep state, subject, and recovery action together in
  a compact fixed status column without a detached raw status column;
- the required-tool health row stays compact, and the diagnostic page has no
  unexplained horizontal overflow at desktop width;
- the browser reports no console errors during the guide and home checks;
- the folder picker source compiles for Windows and contains the Explorer
  `IFileOpenDialog`/`FOS_PICKFOLDERS` contract without `SHBrowseForFolderW`.

No company Jenkins, production release, or destructive cleanup was contacted as
part of verification. Jenkins MCP execution remains covered by the existing
Action Broker approval boundary; the new MCP unit test verifies missing required
arguments are returned as tool errors.
