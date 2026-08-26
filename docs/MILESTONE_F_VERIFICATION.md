# Milestone F verification

Status: partial; dashboard foundation is accepted, trends/export traceability remains deferred
Updated: 2026-08-26

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
