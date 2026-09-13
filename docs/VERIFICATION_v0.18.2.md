# v0.18.2 verification record

Status: **RELEASED — automated, browser, package, CI, and remote-asset gates passed**
Publication: **COMPLETE** — annotated tag `v0.18.2`; release is non-draft and non-prerelease.
Updated: 2026-09-13

This record covers the backward-compatible v0.18.2 candidate and final
publication. It does not replace or modify the historical v0.18.1 record.

## Candidate boundary

| Item | Value |
| --- | --- |
| Release line | `0.18.2` patch |
| Baseline | published `v0.18.1` |
| Working-tree HEAD before release commit | `86a58aa` (`v0.18.1`) |
| Candidate source commit | `6cbd46f1e31c9dad0892b49de3602fc743930e71` |
| Published release commit | `6e2a52ff1f5886adb95cdc50b39c63c4ddf80719` |
| Published tag | `v0.18.2` (annotated object `e599973b771e65c2e13654c4af7b1d7cb4b8e306`) |
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
| Full-run evidence | Retained `verification.log` and `verification-summary.json` (private temporary path omitted) | **PASS** |
| UI CJS regression suite | targeted UI CJS suite, `32/32` assertions | **PASS** |

The Full runner passed normal tests, race tests, vet, module verification,
build, Windows amd64 build, Windows arm64 cross-build, and diff checks. The
arm64 binary is verification-only and is not a release asset. Focused checks
were launched from WSL using the Windows Go toolchain; the Full runner and
browser acceptance below are the native Windows evidence.

## Release acceptance

| Tier | Required evidence | Status |
| --- | --- | --- |
| Windows toolchain | `pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full` with exact versions and retained summary | **PASS** |
| UI CJS regression | Targeted CJS suite | **PASS — 32/32** |
| UI browser regression | Edge 153: all Home, Projects, Work, Assurance, Diagnostics, Activity, and Guide routes; one heading; no console/runtime/HTTP errors or horizontal overflow | **PASS** |
| P1 coverage | Timeout/failure profile is non-active and evidence state/reason are shown separately from retention | **PASS** |
| P1 target state | Non-default Worktree remains selected across discovery/action/external/guidance and localStorage refresh; exact-candidate empty state is explicit | **PASS** |
| P2 journeys | Parent-folder grouping, Assurance first-use CTA, Activity noise, Diagnostics layout, control names | **PASS** |
| Package | `pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.18.2`, archive/version/checksum smoke | **PASS — pre-final local gate** |
| Release acceptance | Exact published commit/tag, CI, Windows amd64 asset hash, and remote release verification | **PASS — CI run `34743438251`** |

### Pre-final local package gate evidence

The first local package gate passed and created exactly these files under
`artifacts/0.18.2`:

- `dev-control-room_0.18.2_windows_amd64.zip` — `13,812,454` bytes
- `SHA256SUMS`

The ZIP contains version `0.18.2`. Its computed SHA-256 is
`6af85d5bd306b8b81ee922c7871fb9030c09aeea3698f8b3005bc99457a776c9`, matching
the generated `SHA256SUMS`. This is pre-final local candidate evidence, not a
published asset hash; the final published asset is recorded below.

## Final publication evidence

- Release: [v0.18.2](https://github.com/knowgyu/dev-control-room/releases/tag/v0.18.2),
  published `2026-09-13T07:01:20Z`, non-draft and non-prerelease.
- Source/tag: commit `6e2a52ff1f5886adb95cdc50b39c63c4ddf80719`, annotated tag
  object `e599973b771e65c2e13654c4af7b1d7cb4b8e306`.
- CI: [run `34743438251`](https://github.com/knowgyu/dev-control-room/actions/runs/34743438251)
  **success**; Linux completed `2026-09-13T06:43:55Z` and Windows completed
  `2026-09-13T07:00:16Z`.
- Published assets are exactly:
  - `dev-control-room_0.18.2_windows_amd64.zip` — `13,812,720` bytes;
    downloaded SHA-256 `bf19ea76591ff1216c12aca319ebc960fb5e7adba22f776906b043fd2a69036d`.
  - `SHA256SUMS` — `109` bytes; the downloaded ZIP hash matches its checksum.
- Extracted native Windows executable version JSON reports `0.18.2`.

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
installation, or destructive cleanup was performed. Tag `v0.18.2`, its push,
release publication, and remote asset verification are recorded above. Browser
acceptance used only the temporary homes listed above. Mutation testing, causal
quality-score evidence, and separate company/provider/second-device acceptance
remain unproven; CI Node20/cache warnings were non-fatal.
