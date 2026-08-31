# Pilot quality loop

This is the durable scorecard for the local quality pilot. It records what was
actually observed at a specific commit; it is not a release checklist, ranking,
mutation score, or productivity score. No aggregate score is calculated or
shown. The manifest schema and full metric rules remain in
[`DOGFOOD_MEASUREMENT.md`](DOGFOOD_MEASUREMENT.md), and the dashboard workflow
remains in [`MEASUREMENT_DASHBOARD.md`](MEASUREMENT_DASHBOARD.md).

## Scorecard

| Metric | Meaning | Default |
| --- | --- | --- |
| `quality.gofmt` | Whether tracked Go files are formatted; duration is supporting evidence. | Required |
| `quality.go.test` | Normal Go test gate result and duration. | Required |
| `quality.go.test_race` | Uncached race-enabled Go test gate result and duration. | Required |
| `quality.go.vet` | Static `go vet` gate result and duration. | Required |
| `quality.go.mod_verify` | Go module integrity result. | Required |
| `quality.go.build` | Repository build result. | Required |
| `quality.ui.syntax` | Embedded UI JavaScript syntax result. | Required |
| `quality.go.coverage` | Whether the optional coverage collection process completed; duration is supporting evidence. | Optional |
| `quality.go.coverage_percent` | Total statement coverage when the optional profile is valid. | Optional |
| `performance.http.health.latency` | Bounded latency samples for an explicitly selected loopback `/api/health` probe. | Optional |
| `performance.http.state.latency` | Bounded latency samples for an explicitly selected loopback `/api/state` probe. | Optional |
| `process.dogfood.run_duration` | Total duration of the measurement runner. | Informational |

Use one status per metric or evidence item:

- `PASS`: the fixed check ran and met its success rule.
- `FAIL`: the check ran and did not meet its success rule.
- `NOT_RUN`: the check was intentionally not attempted.
- `UNAVAILABLE`: the source or required evidence was not available.
- `INCONCLUSIVE`: the run produced evidence, but it is insufficient or
  contradictory for a defensible conclusion.

`unknown` in the v1 manifest is an evidence state, not a pass. Map it to
`NOT_RUN`, `UNAVAILABLE`, or `INCONCLUSIVE` in the scorecard according to the
reason, and keep the reason visible. Do not turn optional evidence into a
passing result. `quality.go.test_race` must use `-count=1`; cached race output
does not count as a fresh pilot observation.

## Comparison rule

The dashboard may calculate a current-minus-previous delta only when all of
these conditions hold:

1. Both manifests validate as `devroom/measurement/v1` runs.
2. Both runs have the same commit, HEAD, OS, architecture, configuration
   digest, and complete tool-version key/value set.
3. Both runs report `dirtyState: clean`. A dirty run is never a baseline; two
   dirty runs are explicitly `incomparable`.
4. The previous run ended before the current run.
5. For an individual metric, ID, name, category, unit, and command ID match;
   both records are `pass`/`measured` and both have a p50. A p95 delta is shown
   only when both p95 values exist.

If identity is missing or unknown, show `unknown`. If prior evidence exists but
fails any compatibility condition, show `incomparable`. If no prior run exists,
show `missing`. Never substitute the latest dirty run or a mismatched commit as
a baseline.

## Commands

Run from the checkout being measured. The runner is read-only and writes only
to the selected output directory:

```powershell
pwsh -NoProfile -File .\scripts\measure-dogfood.ps1 -OutputDirectory .\artifacts\dogfood
pwsh -NoProfile -File .\scripts\verify-measurement-contract.ps1 -ManifestPath .\artifacts\dogfood\dogfood-measurement.json
pwsh -NoProfile -File .\scripts\measure-dogfood.ps1 -OutputDirectory .\artifacts\dogfood -ProbeServer -ServerUri http://127.0.0.1:38471 -RequestCount 5
```

The fixed source gate and proportional verification commands are:

```powershell
gofmt -l (git ls-files '*.go')
go test -count=1 ./...
$env:CGO_ENABLED = "1"; go test -count=1 -race ./...
go vet ./...
go mod verify
go build ./...
node --check internal/app/ui/app.js
git diff --check
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full
```

The Go fuzz target may be smoke-tested with a short bound; this is test
evidence, not a completed campaign or a score:

```powershell
go test ./internal/app -run '^$' -fuzz=FuzzMeasurementManifestValidationAndComparison -fuzztime=2s
```

Use the exact commit, environment, scope, command exit codes, artifacts, and
gaps when recording evidence. WSL, cross-build, fake-provider, or a successful
local check does not claim native Windows, provider-authoritative, adoption, or
causal evidence; see [`VERIFICATION_PLAYBOOK.md`](VERIFICATION_PLAYBOOK.md).
