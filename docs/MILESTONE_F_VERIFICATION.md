# Milestone F verification

Status: accepted for automated usage/effects/pricing scope
Updated: 2026-08-26

The assurance dashboard filters invocations by provider/model, aggregates only
reported usage, and leaves cost unknown when usage or an immutable pricing
snapshot is missing. Known costs are labelled `estimated public API list-price
equivalent`; they are not Enterprise entitlement billing. Effect fingerprints
deduplicate measured outcomes, and historical pricing snapshots reject mutation.

```text
scope: WSL; isolated generic Git fixture
commands:
  go test ./internal/app ./internal/store -run 'TestEffectsAndUsage|TestAllV1Technique' -count=1  PASS
```

gaps: native dashboard screenshots, external provider billing, and real
Enterprise usage fields were not exercised. No pricing URL was contacted.

