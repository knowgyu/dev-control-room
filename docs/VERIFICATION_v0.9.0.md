# v0.9.0 verification record

Status: release candidate verified on a Windows host; remote release-asset
download verification is performed after publication.

Candidate source: v0.9.0 release candidate  
Date: 2026-08-26  
Environment: native Windows, PowerShell 7.6, Go, Node, Git, and the embedded
loopback browser runtime.

## Automated verification

| Command | Result | Evidence |
| --- | --- | --- |
| `go test ./internal/app ./internal/domain ./internal/store ./cmd/dev-control-room` | PASS | Existing application, domain, store, CLI, fake-Provider, and Phase 1/2 regression tests. |
| `go test -count=1 -race ./...` | PASS | Race detector passed for every package. |
| `go vet ./...` | PASS | No vet findings. |
| `go build ./...` | PASS | All Go packages build. |
| `go mod verify` | PASS | All modules verified. |
| `node --check internal/app/ui/app.js` | PASS | Embedded UI JavaScript parses successfully. |
| `pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1` | PASS, 239 assertions | Fresh temporary homes and Git fixtures; first-use, return, Provider recovery, local npm launcher, fake Provider, local Quality runner, blocked paths, restart, duplicate warning, and secret-canary checks. |
| `pwsh -NoProfile -File .\scripts\verify-real-codex.ps1` | PASS, 46 assertions | Fresh local fixture; authenticated local Codex invocation through typed `node.exe` and the verified `@openai/codex/bin/codex.js` entrypoint. |
| `go test ./internal/app -run 'TestAssurance(ImpactAndTraceExposeMeasuredEvidenceWithoutLocalPaths|ArtifactArchiveManifestPinAndRestore|ImpactHTTPContracts|ArtifactRetentionRejectsCorruptLocalFile)|TestEmbeddedUI(AssuranceDashboardContract|Assets|Routes)$' -count=1` | PASS | Effect period metrics, safe trace, report contracts, manifest/pin/restore, quota/hash boundary, and embedded UI contracts. |
| `go test ./cmd/dev-control-room -run 'TestAssurance(CLIHelpListsLifecycleCommands|ArtifactCLIExportsVerifiedPack|CLI.*)' -count=1` | PASS | Artifact lifecycle CLI help and verified evidence-pack export. |
| `pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.9.0-final` | PASS | Final gofmt, full test, UI syntax, race, vet, module verification, package build, Windows amd64/arm64 cross-build, and diff check. |

The final verifier records the exact Windows build hashes in
`artifacts/verification-v0.9.0-final/verification-summary.json` and
`verification.log`.

The final verifier summary and log record the exact Windows build hashes
together with the source commit. Those generated files, plus the release
`SHA256SUMS` asset, are the publication source of truth; this tracked record
does not duplicate hashes that change when Git version metadata changes.

## Browser, UI, and keyboard evidence

- The Assurance screen was opened on a local loopback server with a disposable
  project, fake Provider invocation, local Quality Run, measured effect, and
  linked artifact.
- Desktop and 390px narrow screenshots are captured at
  `artifacts/verification-v0.9.0-browser/assurance-dashboard-desktop.png` and
  `artifacts/verification-v0.9.0-browser/assurance-dashboard-mobile.png`.
- The visible screen shows the value headline, selected period and scope,
  previous-period state, metric provenance, trend, data quality,
  traceability score, Quality result, effect record, storage state, and
  artifact SHA-256.
- Trace drill-down opens nodes, typed links, and artifact evidence; closing the
  panel restores the page state. The filter can be changed and reset through
  the browser controls; the local browser keyboard driver successfully focused
  the filter/reset controls and activated reset with Enter.
- The page contained no local storage path and the browser console error list
  was empty. CSV/JSON export buttons reached their success notices and the
  downloadable API returned the report contract; this in-app Browser runtime
  did not expose a Blob download event, so that event is not claimed as an
  automated pass.

## Effect and artifact proof

The fixture flow recorded a measured effect with an adopted and reverified
commit, linked source invocation, artifact SHA-256, Trace ID, and 12.5-minute
value. The impact view reported the effect, adoption and reverification rates,
time-saved metric, evidence completeness, and an unavailable comparison when a
previous-period sample was absent. The artifact tests also verified archive
manifest hash, pin protection, restore, unpin, delete, and corrupt-local-file
rejection.

## Native Windows and provider boundary

The verifier ran on Windows and produced portable amd64/arm64 binaries. The
Phase 2 fixture exercised a real local typed `node.exe` invocation of a
synthetic `@openai/codex/bin/codex.js`; the real-Codex fixture also exercised
the authenticated local entrypoint, and no shell launcher was executed. The
full native interactive smoke procedure and company endpoint checks are not
repeated by this record unless explicitly available in the local environment.

Package verification passed after fresh extraction: the amd64 binary reported
version `0.9.0`, returned a successful JSON help envelope, and served a
loopback-only health response. The release `SHA256SUMS` asset is the
publication source of truth for the final portable-archive hashes; verify it
with `Get-FileHash` after download.

## Not run / not accepted

- Company GitHub/Jenkins/Kubernetes endpoints, production, proxy, credential
  mutation, or destructive cleanup.
- Native mutation-tool execution when the tool is unavailable.
- Provider crash, expired auth, closed-stdin, approval-prompt, process-tree
  cancellation, and idempotent resume acceptance.
- Patch materialization, human commit/push, and causal/productivity attribution.
- Full Tab/Space traversal, reliable native-dialog Esc automation,
  assistive-technology review, and a second clean Windows device.
- Automatic Blob download-event observation in the in-app Browser runtime; the
  UI success path and direct report endpoint were verified separately.

These are boundaries, not hidden failures. They remain reflected in the
Assurance plan, ADR, and handoff documents.
