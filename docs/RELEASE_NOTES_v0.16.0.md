# Dev Control Room v0.16.0

Draft release notes. The source version is now `0.16.0`; the release is not
published and final release verification remains pending.

v0.16.0 is the quality-first pilot persistence and measurement-safety slice
from `v0.15.2..e216acb`.

## Included

- Assurance continuity hardening: the existing lease, restart-to-`interrupted`,
  and explicit-retry contracts now have stronger atomic writes, failure
  compensation, and repeated-restart evidence preservation.
- Evidence safety: fail-closed cleanup preserves referenced artifacts and
  removes only safe crash orphans.
- Measurement safety: bounded canonical Go manifest validation, strict decoding,
  explicit `unknown`/`unavailable` partial-probe states, temporary-file
  validation before replacement, and preservation of the last valid manifest
  when validation rejects a new one.
- Pilot UI polish: improved responsiveness, focus behavior, keyboard table
  scrolling, import status accessibility, and embedded UI line-ending
  normalization.
- Expanded regression coverage for restart, retry, finalization, cleanup,
  measurement contracts, fuzzing, and UI contracts.

## Verification boundary

The 2026-09-07 native Windows resume run passed all 10 gates, and the script
worker passed Pester 6/6. Those results were recorded before the version bump
on a dirty worktree rooted at `2a95404` with reviewed changes; they are
regression evidence, not proof that a clean v0.16.0 candidate is fully tested.

Clean dogfood, the v0.16.0 binary/package smoke, browser acceptance, and
publication remain pending. Go 1.23 runtime execution, the mutation campaign,
and a causal quality score were not run or established.

## Release assets

The release target is one Windows amd64 ZIP plus `SHA256SUMS`. Windows arm64 is
verification-only; Linux and arm64 release packages are not published.
