# Dogfood measurement

This slice defines a reproducible, local evidence record for running the fixed
quality gate against Dev Control Room itself. It is a measurement manifest, not
a quality score, ranking, release claim, or causal productivity claim.

## First run

From a native Windows checkout, run:

```powershell
pwsh -NoProfile -File .\scripts\measure-dogfood.ps1 -OutputDirectory .\artifacts\dogfood
```

To probe an already-running loopback server, add a bounded request count:

```powershell
pwsh -NoProfile -File .\scripts\measure-dogfood.ps1 -OutputDirectory .\artifacts\dogfood -ProbeServer -ServerUri http://127.0.0.1:38471 -RequestCount 5
```

The runner writes `dogfood-measurement.json` and
`dogfood-measurement-report.md`. When coverage succeeds it also keeps a
run-specific `coverage-<run-id>.out` profile. Output under `artifacts/` is
ignored by the repository. The runner does not edit source files, install
tools, contact external services, or run arbitrary commands.

The optional static contract check is:

```powershell
pwsh -NoProfile -File .\scripts\verify-measurement-contract.ps1 -ManifestPath .\artifacts\dogfood\dogfood-measurement.json
```

The runner exits non-zero only when a required fixed check does not pass (or
the runner cannot complete its own measurement). Coverage and server probing
are optional; an unavailable optional source is recorded as `unknown` with
`unavailable` provenance and does not turn a successful required gate into a
pass for that source.

## Contract

Both the run and every individual measurement use
`apiVersion: devroom/measurement/v1`. The top-level kind is
`DogfoodMeasurementRun`; individual records use `Measurement`.

The run contains a required-check status, required failure IDs, the
reproducibility envelope, and an array of individual measurements. The run
status is only the fixed required-check gate. It is not an aggregate quality
score and must not be converted into one.

Each measurement contains a stable name, category, status, provenance, unit,
sample count, bounded raw samples, `min`, `p50`, `p95`, `max`, optional
`baseline` and `delta`, a fixed command identity, and an optional exit code.
Summary values are absent (`null`) when no valid sample exists. A sampled
record must have a summary consistent with its raw samples.

The reproducibility envelope contains run ID, commit, HEAD, dirty state,
operating system, architecture, bounded tool-version strings, configuration
digest, and UTC start/end timestamps. It deliberately has no repository path,
output path, secret, raw command output, or arbitrary absolute path.

## Metric catalog

| Measurement name | Category | Unit | Source | Required | Meaning |
| --- | --- | --- | --- | ---: | --- |
| `quality.gofmt` | quality | milliseconds | fixed `gofmt -l` result | yes | Formatting check duration; any unformatted file fails the check. |
| `quality.go.test` | quality | milliseconds | fixed Go test process | yes | Normal Go test gate duration and exit status. |
| `quality.go.test_race` | quality | milliseconds | fixed Go race-test process | yes | Race-enabled Go test gate duration and exit status. |
| `quality.go.vet` | quality | milliseconds | fixed `go vet` process | yes | Static vet gate duration and exit status. |
| `quality.go.mod_verify` | quality | milliseconds | fixed `go mod verify` process | yes | Module integrity gate duration and exit status. |
| `quality.go.build` | quality | milliseconds | fixed `go build ./...` process | yes | Repository build gate duration and exit status. |
| `quality.ui.syntax` | quality | milliseconds | fixed Node syntax check | yes | Embedded UI JavaScript syntax gate duration and exit status. |
| `quality.go.coverage` | quality | milliseconds | optional coverage test process | no | Coverage collection attempt; it does not gate the run. |
| `quality.go.coverage_percent` | quality | percent | optional `go tool cover` summary | no | Measured total statement coverage when the profile and summary are valid. |
| `performance.http.health.latency` | performance | milliseconds | bounded loopback GET | no | Latency samples for `/api/health`. |
| `performance.http.state.latency` | performance | milliseconds | bounded loopback GET | no | Latency samples for `/api/state`. |
| `process.dogfood.run_duration` | process | milliseconds | runner stopwatch | no | Total runner duration. |

The `runtime` category is reserved for future bounded runtime observations; it
does not imply that a runtime metric exists in this slice.

## Status and provenance rules

Status and provenance are independent dimensions:

- `pass` means the measured check or observation met its fixed success rule.
- `fail` means a measured process or HTTP response did not meet that rule.
- `unknown` means the result cannot be decided from the available evidence.
- `measured` is reserved for an executed fixed process or completed HTTP
  response with recorded samples.
- `estimated` is a human estimate and is never emitted by this runner.
- `inferred` is a labelled interpretation and is never treated as measured
  evidence.
- `unavailable` means the source could not supply a defensible measurement,
  such as an absent local server or an unavailable optional coverage path.

An unknown or unavailable measurement is not a pass. The runner preserves raw
latency/duration samples up to 128 values and rejects non-finite or unbounded
sample data in the Go contract. Command output is intentionally discarded
from exported records so test output cannot carry secrets, transcripts, or
absolute paths across the evidence boundary.

## Baselines and deltas

`baseline` and `delta` are optional numeric fields. They are populated only
when a caller has a known baseline in the same unit and a comparable
configuration; `delta` means current value minus baseline. This runner does
not invent a baseline and does not populate either field. A baseline from a
different commit, tool version, request count, endpoint, or configuration
digest is not a comparable baseline and should remain unknown.

Latency p95 is meaningful only when it is calculated from repeated,
comparable samples under a stated configuration. A one-sample command duration
has a p95 equal to that sample, but it is not evidence of a stable tail.

## Evidence boundaries and known gaps

The runner is intentionally a small dogfood foundation. It does not provide a
mutation adapter, property/fuzz campaign, provider-authoritative CI result,
artifact persistence, human patch adoption, or causal quality attribution.
Mutation is not real evidence unless a reviewed native mutation runner is
available and its report is preserved. A tool name or an inferred score is not
an executed mutation result. Likewise, a missing local server is unavailable,
not a healthy latency result.

The first run proves only the recorded native PowerShell/Go/Node environment at
the exact captured HEAD. WSL, a cross-build, a fake server, or a successful
local test cannot be promoted to native Windows UI, provider, production, or
release acceptance. Repeat the run after changing source, tools, configuration,
server request count, or the selected endpoint.
