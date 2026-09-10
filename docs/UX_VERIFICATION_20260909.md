# UX verification — 2026-09-09

## Preliminary candidate iteration

- Candidate URL: `http://127.0.0.1:38481/#home`
- Candidate PID: `29648`
- Candidate home: isolated `artifacts/ux-20260909/isolated-home`
- Fixture: `artifacts/ux-20260909/fixture` (local fixture commit `23e9652df31199cd767a7f981013a11a87366cb2`)
- Scope: unregistered empty flow, then fixture registration and quality-run UX
- Browser harness: CUA/in-app browser only

## Executed results

- Preliminary browser flow: PASS — fixture registration succeeded; reload after
  scan restored the fixture Worktree.
- Real Go vet Quality Run: PASS — run ID `run-6e1c0c2600aae980`.
- During the run: PASS — both launch controls and the target control were
  disabled while execution was active.
- Go coverage Quality Run: PASS — run ID `run-898ee8c8a2baf453`; 100% for the
  tiny fixture's single statement only. This is not main-repository coverage.
- Native Full (first iteration): FAIL — `go test -count=1 ./...` exited 1 after
  `gofmt-check` passed. The preserved evidence is
  `artifacts/ux-20260909/native-full/verification-summary.json` and
  `artifacts/ux-20260909/native-full/verification.log`. The failure was the
  then-current legacy embedded-UI contract set (quality-home queue/error,
  campaign launch, responsive polish, evidence-flow, and first-use markers),
  not a clean full pass.

## Paused checkpoint — 2026-09-10

- Focused current UI gate: PASS — `node --check internal/app/ui/app.js` and
  `go test ./internal/app -run '^TestEmbeddedUI' -count=1` completed successfully.
- A candidate rebuild from that snapshot exists at
  `artifacts/ux-20260909/dev-control-room.exe` (SHA-256
  `EA96A94329141DD50C368229DDA849E6B813574F3830D029023F89C6E9D2296C`).
- The candidate is listening at `http://127.0.0.1:38481/#home` as PID `24800`
  with the existing isolated home `artifacts/ux-20260909/isolated-home`; fixture
  records were preserved. No final Native Full rerun was started after this
  rebuild.
- Final verification is paused for the target-stack scope correction
  (Python/FastAPI + Vue). No release was produced.

## First-iteration issue

The first candidate iteration showed stale UI immediately after the scan
button flow: the UI reported no Worktree/last observation until refresh.
Read-only `/api/state` later confirmed the fixture was healthy (`master`, one
Worktree, scanned at `14:52:39`). This is recorded as a pending UI refresh
issue, not a backend discovery failure.

The browser screenshot also showed a Work layout defect: the target and
technique controls were squeezed into the rightmost roughly 40% of the view.
Descartes is reviewing that CSS/layout issue. The candidate must be rebuilt
after the worker fix before final QA claims.

No target repository registration was performed during the initial empty-flow
check. No runtime/UI files were changed by the QA worker.

## Release/QA preparation checkpoint — 2026-09-10

Preparation is incremental Windows amd64 only and is not publication.
Python/FastAPI + Vue describes the user's repositories; Dev Control Room
remains a Go application. The accepted M1 scope is read-only component and
configuration discovery with display-only setup previews. Python/Vue analyzer
execution belongs to M2.

### Observed source and release state

- Branch: `main`; `HEAD` and `origin/main` are
  `5bded0eee5a03fa5dcaca7c30c5a6978c5521e4c`.
- Worktree: dirty. Existing changes include application/UI/collector files and
  untracked worker files; they were not modified by this QA preparation.
- Source version: `0.16.0` (`internal/version/version.go`). The released
  binary also reports `0.16.0`.
- Existing tag: `v0.16.0` points to
  `804faf0ea8c84ea182cacd9b07322d5b984dc555`, not current `HEAD`.
- Existing release policy: one Windows amd64 ZIP plus `SHA256SUMS`; arm64 is
  verification-only. No tag, package publication, upload, or release mutation
  was performed.

### Release gate checklist

- [x] Target scope clarified: M1 discovers Python/FastAPI, Vue and mixed
  components; it does not install dependencies or execute Python/Vue analysis.
- [ ] Select an exact source SHA and verify the intended version/tag relation;
  require a clean or explicitly approved release worktree.
- [ ] Run the focused checks for the selected stack and record exact tool
  versions. The current repository's prior focused UI gate passed, but it does
  not prove the corrected Python/FastAPI + Vue scope.
- [ ] Native incremental verifier for the edit loop:

  ```powershell
  pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Fast `
    -ArtifactDirectory .\artifacts\release-verification-<version>-fast
  ```

- [ ] Native verifier, from a native NTFS checkout, with a fresh evidence
  directory:

  ```powershell
  pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full `
    -ArtifactDirectory .\artifacts\release-verification-<version>
  ```

- [ ] Require `gofmt-check`, `go-test`, `ui-syntax`, `go-test-race`, `go-vet`,
  `go-mod-verify`, `go-build`, Windows amd64/arm64 cross-builds, and
  `git-diff-check` to pass for this repository's native gate; retain the JSON
  summary, log, and binary hashes.
- [ ] Run the applicable native Windows smoke/browser checklist separately;
  verifier success alone does not prove interactive UI or configured-provider
  behavior.
- [ ] Prepare the amd64-only package command, but do not run it until the
  source SHA and Full gate are accepted:

  ```powershell
  pwsh -NoProfile -File .\scripts\package.ps1 `
    -Version <version> `
    -OutputDirectory .\artifacts\release-candidate-<version>
  ```

- [ ] Confirm the output contains only the Windows amd64 ZIP and `SHA256SUMS`,
  rehash the ZIP, extract it in a fresh directory, run `version`/help smoke,
  and record the exact hash.
- [ ] Obtain separate human authorization before creating a tag, uploading,
  publishing, or performing any remote release mutation.

### Existing failed Full evidence retained

The first Full run remains a real `FAIL`, not a pending or release pass:

- Summary: `artifacts/ux-20260909/native-full/verification-summary.json`
- Log: `artifacts/ux-20260909/native-full/verification.log`
- Source recorded by the runner: `5bded0eee5a03fa5dcaca7c30c5a6978c5521e4c`
- `gofmt-check`: PASS; `go-test -count=1 ./...`: FAIL, exit code `1`.
- The failure was the then-current legacy embedded-UI contract set, including
  retired quality-home queue/error, campaign-launch, responsive polish,
  evidence-flow, and first-use markers. It is not evidence for a release.

### Manifest-only target-stack fixture

Prepared under `artifacts/project-setup-20260910/`:

- `backend/pyproject.toml` — FastAPI/Uvicorn manifest and optional pytest/httpx
  test dependencies.
- `frontend/package.json` — Vue/Vite/Vitest manifest.
- `fixture-manifest.json` — explicit no-install, no-network, no-test,
  unregistered state.
- `README.md` — fixture boundary and future approved-check notes.

Manifest hashes (SHA-256): `fixture-manifest.json`
`8414F274D717078130FF7BF0B0AEF65A092D8359F2A3A3F46114334F80BF6C7E`,
`README.md` `13CF3B8C16C4EC2C570A2FD6675B96DBD8B6340CE0D17E76241FEA2E219F389B`,
`backend/pyproject.toml`
`E2488D2CF1D8605322C51736A1E33AB6BB3AF4CC80A5564336D097D0D04A8492`, and
`frontend/package.json`
`47128B13DAD5FBF9DF325B37C53F1C253C8BE42DB0029594F0D1E7ED27325CCC`.

No dependency installation, network access, test execution, registration, or
release publication was performed for this fixture.

## QA fixture and M1 wait checkpoint — 2026-09-10

- The exact folder `artifacts/project-setup-20260910` is now an independent
  local Git fixture. It has local test identity only, branch `master`, clean
  status, no remotes, and commit
  `7b3d12905154dcc6edca8c1dd26696485df69cd1`.
- No dependency installation, network access, push, or remote registration was
  performed.
- M1 readiness is pending: the shared UI currently references
  `/api/quality/setup` and contains setup-surface tests. The API/service/web
  route and its setup test are now present, but the detector package is still
  missing. Focused M1 tests were therefore not started.
- Candidate `38482` with a new isolated home was not built or launched. The
  prior candidate/home and its records remain preserved. Native Full was not
  rerun, and no release gate was asserted.

Latest ownership checkpoint: detector delivery is assigned to Luna/Fermat
`01a086c2-73fc-7be3-ba9d-3010500475de`, and the additive API/service/web
delivery is assigned to Beauvoir
`01a086c2-bae3-7042-807b-df50b2feedf3`. At the latest read-only poll,
`internal/app/quality_setup.go`, its setup test, and the `GET /api/quality/setup`
route were present, but `internal/qualitysetup` was still missing; no build or
focused test was started.

## Native QA resumed — 2026-09-10 21:34 +09:00

This section supersedes the earlier missing-package checkpoints. Both the
detector package and API files now exist.

- Fixture verified: the Git root is exactly
  `artifacts/project-setup-20260910`, branch `master` is clean, HEAD is
  `7b3d12905154dcc6edca8c1dd26696485df69cd1`, and no remote is configured.
  Reinitialization was unnecessary.
- Source base: `5bded0eee5a03fa5dcaca7c30c5a6978c5521e4c`, dirty shared
  worktree under concurrent backend review.
- Fresh QA directory:
  `artifacts/project-setup-qa-20260910/run-20260910-213416`.
- Initial Windows amd64 candidate build: FAIL, exit 1. The detector
  `componentData` definitions did not match field use (`files`,
  `scriptTools`, and map value types); parser types `packageInfo`,
  `requirementsInfo`, `tomlInfo`, and `setupCFGInfo` were undefined.
  Exact compiler output and source hashes are retained in
  `build-attempt-1.log` and `build-attempt-1-source.json` in the QA directory.
- At this checkpoint, port 38482 has not been launched and Native Full has
  not started. The historical first Full failure remains preserved above.
- Build attempt 2 after detector edits: FAIL, exit 1. Remaining errors are
  `setup.go:1008` (boolean AND with string-valued script entry) and
  `setup.go:1013` (non-boolean if condition). Full output is preserved in
  `build-attempt-2.log`. QA made no detector/API/UI changes.
- Main clarified that ongoing detector/UI corrections and a separate 0.17.0
  version/documentation update are still in progress. Any early 38482 build
  and Full run are preliminary. A final gate requires main's explicit freeze
  signal followed by verification of that source and version.

Main owns the browser acceptance on the new candidate:

| Flow | Required observation | Current result |
| --- | --- | --- |
| Empty home | New home has no registered repositories before main acts | PASS reported by main on preliminary PID 8992 |
| Mixed fixture | Backend Python/FastAPI and frontend Vue components with evidence | Preliminary PASS reported by main |
| Manual preference/reload | Selected languages survive reload with unchanged evidence | Not run |
| Unchanged rescan | Same HEAD/config digest preserves the preference | Not run |
| Changed evidence | HEAD/config digest change resets to automatic detection | Not run |
| Seven routes | Route navigation, headings, desktop/narrow geometry, console errors | Preliminary PASS at actual 1244px and 485px; main report |
| M1 boundaries | Previews do not install or execute; configuration is distinct from readiness | Preliminary mixed fixture showed previews only and no Go action; main report |

Earlier 38481 browser observations from main remain preliminary: all seven
routes fit the actual 485px inner width; console error log was empty. The
requested 390px size rendered at 485px, so 390px is unverified. Coverage result
navigation reached the correct hash/target, with sticky clipping requiring a
CSS retake. Registration used the path field; the OS folder picker was not
exercised. Those observations do not establish M1 acceptance on 38482.

### Preliminary M1 candidate available — 21:39 +09:00

- Build attempt 3: PASS, exit 0, Windows amd64 with CGO disabled:
  `go build -trimpath -o <QA-directory>/dev-control-room.exe ./cmd/dev-control-room`.
- URL: `http://127.0.0.1:38482/#home`; hidden PID `8992`.
- Home: `artifacts/project-setup-qa-20260910/run-20260910-213416/isolated-home`.
- Binary version: `0.16.0`; SHA-256:
  `5B34BAA3503CC597A920CF4D77E4988FEF34AAD7AB04B7E1824AAAFD12CE31F2`.
- Health: HTTP 200, envelope `ok=true`; state contained 0 projects when the
  URL was handed to main, rechecked at 21:41:08 +09:00. QA registered none.
- Source remained dirty and unfrozen. This is a preliminary browser candidate,
  not the final 0.17.0 source or release gate.
- Existing ports/homes 38480 and 38481 were not stopped or changed by this
  resumed QA work.

Executed focused results in the same QA directory:

| Command | Result |
| --- | --- |
| `node --check internal/app/ui/app.js` | PASS, exit 0 |
| `go test ./internal/qualitysetup -count=1` | FAIL: `TestInspectMixedBackendFrontend`, `setup_test.go:55`, backend package manager empty, expected pip; `focused-detector.log` |
| `go test ./internal/app -run '^(TestQualitySetup\|TestEmbeddedUI)' -count=1` | FAIL: `TestEmbeddedUIQualitySetupKeepsEvidenceAndGatesGoExecution`, missing `component.checks.filter(check => qualitySetupCheckIsRelevant(component, preference))`; `focused-app-ui.log` |
| Detector retry after changed source | FAIL to compile tests: `setup_test.go:8:2: "net" imported and not used`; `focused-detector-attempt-2.log` |

Main reported the additive API/collector freeze and separate focused API,
collector, discovery and registration passes at 21:41. Detector/UI corrections
were still in progress. Native Full has not started at this checkpoint.

The third detector focused attempt compiled but failed
`TestInspectConservativePythonDeclarations/setup_reset_dependency_scope`
at `setup_test.go:325`: result `frameworks=[] partial=true` with an
unsupported/malformed setup.cfg warning; expected `fastAPI=false partial=false`.
The output is preserved as `focused-detector-attempt-3.log`.

Binary build metadata independently confirms `vcs.revision`
`5bded0eee5a03fa5dcaca7c30c5a6978c5521e4c`, `vcs.modified=true`,
Go `1.26.7`, `GOOS=windows`, `GOARCH=amd64`, and `CGO_ENABLED=0`.

### Main's preliminary browser report

Main confirmed empty home, discovery of exactly one nested mixed fixture,
registration with the name omitted, and automatic scan completion without a
reload. Work displayed backend Python/FastAPI and frontend JavaScript/Vue.
Only previews were offered; no Go action appeared for that fixture.

All seven routes had one visible h1 and no overflow at actual inner widths
1244px and 485px; console error log was empty. A 390px override rendered at
485px, so no 390px claim is made. These are results on the early 0.16.0
candidate; guide/screenshot evidence and UX retakes remain preliminary.

Main also reported Hegel's 0.17.0 version/docs/MCP-version-test freeze and
focused MCP/parser/static passes. QA did not modify those files. Detector/UI
freeze is still required before the final rebuild and Native Full.

Subsequent checkpoint: main reported Fermat's detector freeze with focused
tests, vet, build, formatting and diff checks passed; changes were confined to
`internal/qualitysetup/setup.go` and `setup_test.go`, with no new dependencies.
Banach's final UI freeze remains pending. Main will commit the frozen source
before final Native Full. QA will wait for that explicit signal and verify its
clean SHA; during Full, QA will write artifacts only and leave tracked
verification documentation untouched until completion.

### Second local fixture prepared

- Exact root: `artifacts/project-setup-rootmix-20260910`.
- Independent Git commit: `2e7f855ef905ae3177887d967713c311f632c8b7`;
  branch `master`, clean, no remote, local QA test identity.
- Files: same-root `pyproject.toml` (FastAPI manifest), `go.mod`,
  `sum.go`, `sum_test.go`, and `README.md`.
- Go function has one statement and no external dependencies. The test uses
  named positive, negative, and zero cases with the standard testing package,
  following the Go testing skill's observable-behavior guidance.
- `gofmt -l sum.go sum_test.go`: PASS with no output. Fixture tests/analyzers
  were not executed during preparation. No install/network/remote operation.
- Main owns UI registration and manual Python-suppresses-Go / automatic-Go
  execution acceptance. Those flows have not yet been reported.
- The original nested fixture remains at
  `7b3d12905154dcc6edca8c1dd26696485df69cd1`, clean with unchanged
  backend/frontend manifest hashes. It is retained for the stale-config test.
