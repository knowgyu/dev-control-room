# Dev Control Room v0.16.0

Prepublication candidate record. The source version is now `0.16.0`. The clean
candidate measurement is complete; package, CI, and publication evidence remain
pending.

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
historical regression evidence rather than the clean candidate record.

The clean candidate at `8e9e6e24c9ee3b3896f13abd7a93202ecc45ed38` recorded
dogfood run `dogfood-e125339045844519a9e99a79e21142d5`, required status `pass`,
58.7% statement coverage, health/state probes at 5/5 successes, validator exit
0, measurement import 201, and a matching dashboard run ID. The 373-assertion
journey and 15-assertion native resilience checks also passed. Current browser
acceptance passed seven routes with one `h1` each, main focus, no overflow at
1244px and 485px CSS widths, guide slide 2 surviving reload, zero console
errors, and real-repository registration/read-only scan. The 390px override
rendered at 485px actual width; no 390px claim is made.

The populated browser measurement dashboard confirmed the exact run ID,
required-gate `PASS`, 58.7%, and both HTTP probes at 5/5.

The first Windows CI attempt did exercise the Go 1.23.0 toolchain, but its
aggregate result was not release evidence: PowerShell masked native failures,
and the log exposed path-fixture and storage-test failures. The follow-up
workflow and test-fixture corrections make no production-code changes; successor
CI, binary/package smoke, and publication remain pending. Mutation testing and
a causal quality score remain unrun/unestablished.

## Release assets

The release target is one Windows amd64 ZIP plus `SHA256SUMS`. Windows arm64 is
verification-only; Linux and arm64 release packages are not published.
