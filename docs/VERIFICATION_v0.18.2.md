# v0.18.2 verification record

Status: **PENDING — patch preparation only**
Updated: 2026-09-13

This is a verification scaffold for the backward-compatible v0.18.2 patch.
It does not replace or modify the historical v0.18.1 record. No v0.18.2 tag,
package, or published release is claimed.

## Candidate boundary

| Item | Value |
| --- | --- |
| Release line | `0.18.2` patch |
| Baseline | published `v0.18.1` |
| Working-tree HEAD before release commit | `86a58aa` (`v0.18.1`) |
| Candidate source commit | **PENDING** |
| Runtime target | Windows 11 amd64 |
| Release assets | one Windows amd64 ZIP plus `SHA256SUMS` |
| Linux, macOS, arm64 assets | not release targets |

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

## Checks observed during preparation

These are focused working-tree checks, not release acceptance:

| Scope | Command | Result |
| --- | --- | --- |
| Windows Go toolchain | `go test ./internal/domain ./internal/app -run 'TestQualityCoverageArtifactEvidenceClassifiesIncompleteProfiles\|TestInvalidArtifactCannotCompleteAssuranceEvidence\|TestEmptyArtifactEvidenceStateRemainsValid\|TestQualityRunPersistsNormalizedGoCoverageAndLinkedArtifacts\|TestQualityRunProcessOutcomeDistinguishesFailureTimeoutAndInconclusive\|TestQualityRunPersistsActualRunnerFailureAndReturnsExecutionError' -count=1` | **PASS** |
| Embedded UI syntax | `node --check internal/app/ui/app.js` | **PASS** |
| Go formatting | `gofmt -l` on touched Go files | **PASS** |
| Full Fast runner | `pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Fast` | **PENDING — interrupted during broad suite; no PASS claim** |
| UI contract after concurrent working-tree changes | `go test ./internal/app -run TestEmbeddedUIResponsiveAccessibilityPolishContract -count=1` | **PENDING — rerun after UI integration** |

The focused checks were launched from WSL using the Windows Go toolchain. They
do not prove native interactive Windows behavior.

## Required verification before release

| Tier | Required evidence | Status |
| --- | --- | --- |
| Windows toolchain | `pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full` with exact versions and retained summary | **PENDING** |
| UI browser regression | All Home, Projects, Work, Assurance, Diagnostics, Activity, and Guide routes; one heading; no console errors or horizontal overflow | **PENDING** |
| P1 coverage | Force a timeout/failure with a profile and verify `partial`/`invalid`, reason, excluded score/evidence, and separate retention | **PENDING** |
| P1 target state | Select a non-default Worktree, run discovery, refresh, and verify target/empty state stays bound | **PENDING** |
| P2 journeys | Parent-folder grouping, Assurance first-use CTA, Activity noise, Diagnostics layout, control names | **PENDING** |
| Package | `pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.18.2`, archive/version/checksum smoke | **PENDING** |
| Release acceptance | Exact candidate SHA, Windows amd64 asset hash, remote tag/release verification | **PENDING** |

## Native browser checklist

Run from a fresh temporary `--home` on Windows 11. Do not use the default
application data directory. Confirm the selected real Worktree is displayed
consistently in Work, discovery, action, external, guidance, and Assurance.
Exercise loading/disabled states, keyboard focus return after dialogs and
requests, URL state, and an explicit empty discovery result. Confirm the
coverage artifact status never presents a failed or timed-out profile as active
evidence. Record screenshots or a concise observation log without secrets or
personal paths.

## Safety boundaries

No production or Jenkins endpoint, Scheduler installation, package-manager
installation, destructive cleanup, release tag, push, or release publication
was performed as part of this scaffold.
