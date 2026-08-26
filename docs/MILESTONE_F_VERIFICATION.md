# Milestone F verification

Status: partial; effect-evidence hardening is focused-verified, while patch
adoption and causal attribution remain deferred
Updated: 2026-08-27

The assurance dashboard filters invocations by provider/model, aggregates only
reported usage, and leaves cost unknown when usage or an immutable pricing
snapshot is missing. Known costs are labelled `estimated public API list-price
equivalent`; they are not Enterprise entitlement billing. Effect fingerprints
deduplicate measured outcomes, and historical pricing snapshots reject mutation.

The v0.8.0 candidate presents Quality Runs, effects, Agent invocations, and
artifact retention in one progressive-disclosure dashboard. It shows repeat
execution, linked evidence, bounded usage/cost, and no-transcript guarantees
before the detailed manifests. The filter note explicitly preserves the
unfiltered Quality Run/effect/artifact scope.

## v0.10.0 effect-evidence hardening slice

The impact contract now keeps `measured`, `prevented_regression`,
`user_estimated`, `ai_inference`, and `unavailable` distinct. Measured and
user-estimated `time_saved` values are separate metrics, so an estimate cannot
inflate the measured benefit. Trace-ID-only source links participate in
provider/model filtering and trace summaries. A measured or regression-
prevention effect is counted as verified only when its source and live
artifact evidence are linked, adoption metadata names the same commit as the
reverification commit, and the referenced run or invocation succeeded at that
exact HEAD. The trace response exposes source/reverification nodes and commit
links for review.

Focused checks completed on the working tree:

```text
go test -count=1 ./internal/app -run 'TestAssuranceImpact|TestEffectVerification|TestEmbeddedUIAssuranceDashboardContract' -v  PASS
go test -count=1 ./cmd/dev-control-room -run 'Test.*Help|Test.*Doctor|Test.*Assurance' -v  PASS
go test -count=1 ./...  PASS
node --check internal/app/ui/app.js  PASS
gofmt -l <changed Go files>  PASS
git diff --check  PASS (line-ending warnings only)
```

The v0.10.0 candidate Full verifier passed on the release source. Windows
package smoke and release evidence are recorded from the final package
artifacts. Native process resilience, automatic patch materialization, human
adoption ceremony, and causal attribution remain partial by design.

```text
scope: WSL; isolated generic Git fixture
commands:
  go test ./internal/app ./internal/store -run 'TestEffectsAndUsage|TestAllV1Technique' -count=1  PASS
```

gaps: a local Browser dashboard observation exists, but full keyboard
traversal, non-empty real-provider dashboard data, external provider billing,
and real Enterprise usage fields were not exercised. No pricing URL was
contacted.

## v0.8.0 status update

The dashboard is now included in the clean-state journey and Browser return
view, including its explicit no-transcript and unknown-cost states. It remains
a current-state summary, not a proof of historical trends or exportable effect
claims. Those traceability/export capabilities are
[#12](https://github.com/knowgyu/dev-control-room/issues/12).
