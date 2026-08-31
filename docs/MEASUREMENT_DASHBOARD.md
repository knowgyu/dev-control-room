# Measurement dashboard workflow

The Assurance screen has two intentionally separate surfaces:

- the existing example board, which is static and never contributes to stored
  evidence; and
- the actual Measurement dashboard, which only renders an imported
  `devroom/measurement/v1` `DogfoodMeasurementRun` manifest.

The dashboard is a gate-and-evidence view, not a quality score, ranking, or
causal productivity claim. It does not invent a score when a check or probe is
unknown.

## Workflow

1. Run the fixed native Windows runner from the checkout you want to measure:

   ```powershell
   pwsh -NoProfile -File .\scripts\measure-dogfood.ps1 -OutputDirectory .\artifacts\dogfood
   ```

   The runner records the required Go/Node checks, bounded samples, commit,
   HEAD, dirty state, platform, tool versions, configuration digest, and UTC
   timestamps. It writes `dogfood-measurement.json` and a separate human-
   readable report. The report is not automatically imported as evidence.

2. If a loopback Dev Control Room server is already running, opt into the
   bounded HTTP probe explicitly:

   ```powershell
   pwsh -NoProfile -File .\scripts\measure-dogfood.ps1 `
     -OutputDirectory .\artifacts\dogfood `
     -ProbeServer -ServerUri http://127.0.0.1:38471 -RequestCount 5
   ```

   Without `-ProbeServer`, the optional HTTP measurements remain
   `status: unknown` with `provenance: unavailable`. They are not converted to
   healthy latency values.

3. Open `검증 → 실제 측정 대시보드` and use the browser file picker to choose
   `dogfood-measurement.json`. The browser submits the manifest JSON; there is
   no server-side path parameter, arbitrary file read, command execution, or
   raw command-output import.

4. Confirm the required gate, then inspect the measured summaries and
   reproducibility metadata. Importing an existing run ID again is rejected as
   a duplicate instead of silently replacing evidence.

5. Repeat the runner after a source/tool/configuration change. The dashboard
   compares the latest run with the prior run only when commit, HEAD, dirty
   state, configuration digest, platform, and tool-version identity are all
   known and compatible. A comparison delta is current p50 minus previous p50;
   p95 is shown when both runs provide it. A different commit, tool version,
   request configuration, endpoint, or unknown identity is not a comparable
   baseline.

## Read-only API surface

The UI uses the following local application-service endpoints:

| Method | Endpoint | Purpose |
| --- | --- | --- |
| `GET` | `/api/assurance/measurement-runs` | List safe run summaries |
| `GET` | `/api/assurance/measurement-runs/{runId}` | Get one safe run summary |
| `GET` | `/api/assurance/measurement-runs/dashboard` | Select latest/prior comparable run and evidence-only next actions |
| `POST` | `/api/assurance/measurement-runs/import` | Import one bounded JSON manifest; requires the local mutation token |

List, detail, and dashboard responses omit raw samples and command text. They
retain status, provenance, sample counts, min/p50/p95/max, optional manifest
baseline/delta, command IDs, required failures, and reproducibility identity.
The v1 manifest has no separate report/evidence identity field, so the UI
states that it was not recorded rather than deriving one from a filename.

## Next actions and gaps

Next actions are deterministic and evidence-only:

- `failed_required_check` points to required failure IDs;
- `unavailable_probe` points to unknown or unavailable measurements;
- `incomplete_reproducibility` asks for complete run identity metadata;
- `missing_comparable_baseline` asks for a same-identity prior run; and
- `regression_comparable_metric` names a comparable duration/latency increase
  or coverage decrease.

The dashboard does not claim mutation coverage, provider-authoritative CI,
artifact persistence for the runner report, patch adoption, or causality. The
runner contract check remains available before import:

```powershell
pwsh -NoProfile -File .\scripts\verify-measurement-contract.ps1 `
  -ManifestPath .\artifacts\dogfood\dogfood-measurement.json
```

For the full manifest rules and metric catalog, see
[`DOGFOOD_MEASUREMENT.md`](DOGFOOD_MEASUREMENT.md).
