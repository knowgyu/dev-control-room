# v0.18.2 verification record

Status: **RELEASE CANDIDATE — automated and browser acceptance passed; package/release pending**
Updated: 2026-09-13

This record covers the backward-compatible v0.18.2 patch candidate. It does
not replace or modify the historical v0.18.1 record. No v0.18.2 tag, package,
or published release is claimed.

## Candidate boundary

| Item | Value |
| --- | --- |
| Release line | `0.18.2` patch |
| Baseline | published `v0.18.1` |
| Working-tree HEAD before release commit | `86a58aa` (`v0.18.1`) |
| Candidate source commit | `6cbd46f1e31c9dad0892b49de3602fc743930e71` |
| Runtime target | Windows 11 amd64 |
| Release assets | one Windows amd64 ZIP plus `SHA256SUMS` |
| Linux, macOS, arm64 assets | not release targets |
| Full gate window | `2026-09-13T15:27:41.7213843+09:00` → `2026-09-13T15:36:08.1480948+09:00` |
| PowerShell / Go / Node / Git / gcc | `7.6.6` / `1.26.7 windows/amd64` / `v24.15.0` / `2.53.0.windows.1` / `16.2.0` |

## Implemented scope to verify

- Assurance AI-candidate checkbox layout reset.
- Artifact `EvidenceState`/`EvidenceReason` separate from retention; failed or
  timed-out coverage is partial, truncated or unparseable coverage is invalid,
  and non-valid evidence is excluded from completeness, score, trace, and
  report aggregation.
- Shared selected Worktree state and discovery-refresh preservation, including
  explicit zero-result state.
- Parent-folder repository/Worktree grouping, Assurance first-use guidance,
  Activity no-op reduction, compact Diagnostics provider rows, and stable
  dynamic control identity.

## Automated verification

| Scope | Command | Result |
| --- | --- | --- |
| Windows Go toolchain | `go test ./internal/domain ./internal/app -run 'TestQualityCoverageArtifactEvidenceClassifiesIncompleteProfiles\|TestInvalidArtifactCannotCompleteAssuranceEvidence\|TestEmptyArtifactEvidenceStateRemainsValid\|TestQualityRunPersistsNormalizedGoCoverageAndLinkedArtifacts\|TestQualityRunProcessOutcomeDistinguishesFailureTimeoutAndInconclusive\|TestQualityRunPersistsActualRunnerFailureAndReturnsExecutionError' -count=1` | **PASS** |
| Embedded UI syntax | `node --check internal/app/ui/app.js` | **PASS** |
| Go formatting | `gofmt -l` on touched Go files | **PASS** |
| Windows Full runner | `pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full` | **PASS** |
| Full-run evidence | `C:\Users\knowgyu\AppData\Local\Temp\dev-control-room-verify-110ad653e7134b8cb5c4f2f0209600d1\verification.log` and `verification-summary.json` | **PASS** |
| UI CJS regression suite | targeted UI CJS suite, `32/32` assertions | **PASS** |

The Full runner passed normal tests, race tests, vet, module verification,
build, Windows amd64 build, Windows arm64 cross-build, and diff checks. The
arm64 binary is verification-only and is not a release asset. Focused checks
were launched from WSL using the Windows Go toolchain; the Full runner and
browser acceptance below are the native Windows evidence.

## Required verification before release

| Tier | Required evidence | Status |
| --- | --- | --- |
| Windows toolchain | `pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full` with exact versions and retained summary | **PASS** |
| UI CJS regression | Targeted CJS suite | **PASS — 32/32** |
| UI browser regression | Edge 153: all Home, Projects, Work, Assurance, Diagnostics, Activity, and Guide routes; one heading; no console/runtime/HTTP errors or horizontal overflow | **PASS** |
| P1 coverage | Timeout/failure profile is non-active and evidence state/reason are shown separately from retention | **PASS** |
| P1 target state | Non-default Worktree remains selected across discovery/action/external/guidance and localStorage refresh; exact-candidate empty state is explicit | **PASS** |
| P2 journeys | Parent-folder grouping, Assurance first-use CTA, Activity noise, Diagnostics layout, control names | **PASS** |
| Package | `pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.18.2`, archive/version/checksum smoke | **PENDING** |
| Release acceptance | Exact candidate SHA, Windows amd64 asset hash, remote tag/release verification | **PENDING** |

## Native browser acceptance

Browser: Microsoft Edge 153 on Windows 11, actual browser engine. The
acceptance used temporary homes only: `dcr-v0182-browser-final-O0rrdB` and
`dcr-v0182-browser-mhqGHW\home`.

- All seven routes (Home, Projects, Work, Assurance, Diagnostics, Activity,
  Guide) rendered exactly one visible `h1`.
- At 1440px and 390px viewport widths, horizontal overflow was `0`; console,
  runtime, and HTTP errors were all `0`.
- First-use CTA was present and did not issue a `/latest` request before user
  action.
- AI checkbox computed layout was `13px` wide with `min-height: 0` and
  `padding: 0`; its description column measured `502px`.
- Duplicate IDs were `0`; loading/disabled behavior was observed as expected;
  route focus returned to `main-content`; Guide slide state and Assurance
  `days=7` state survived reload through the URL.
- A linked Worktree selection remained consistent across Work, discovery,
  action, external, and guidance controls and persisted through localStorage.
- Diagnostics showed compact provider badges without the old provider rail.
  Dynamic selects and checkboxes had `0` unnamed controls.
- Parent-folder discovery showed 9 repositories collapsed under `다른 발견
  저장소`, unchecked by default. Synthetic exact-candidate discovery produced
  the requested Worktree-specific empty state.

## Safety boundaries

No production or Jenkins endpoint, Scheduler installation, package-manager
installation, destructive cleanup, release tag, push, or release publication
was performed. Browser acceptance used only the temporary homes listed above.
Package generation and remote release verification remain pending.
