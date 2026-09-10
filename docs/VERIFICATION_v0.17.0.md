# v0.17.0 verification record

Status: **RELEASE CANDIDATE — NOT PUBLISHED**
Publication: **PENDING — not performed**
Package/archive smoke: **PENDING — not run**
Updated: 2026-09-10

This record describes the implemented M1 candidate. The exact native Full gate
and final browser acceptance passed, but package generation, CI confirmation,
tagging, and publication remain separate release steps.

## Scope and non-claims

The scope is project-oriented, read-only setup inspection for a registered
selected checkout: automatic/manual display language filtering, mixed
Python/FastAPI, Vue, JavaScript/TypeScript, and Go component evidence, bounded
setup previews, and the prior UX and repository-discovery fixes.

M1 does **not** include Python/FastAPI or Vue analyzer execution, normalized
findings, automatic installation, AI setup or source upload, arbitrary shell
execution, or a persisted approved executable plan. Existing Go root-only
actual checks remain the prior supported execution path.

## Candidate identity

| Field | Status | Required follow-up |
| --- | --- | --- |
| Source SHA under Full test | `998325b4297d75be63d51747b33f2beb9e9e6243` | Clean at Full-run start; current docs edits are separate and uncommitted. |
| Implementation commit | `7a112dcd580e1c4116d349bb6cbc64d50f16c92a` | Production implementation included in the candidate history. |
| Branch/worktree | `main`; pushed to `origin/main` for CI | The Full run used the clean source SHA above. |
| Latest published release | `v0.16.0` | Baseline only; not v0.17.0 evidence. |
| Release target | `Windows amd64 ZIP + SHA256SUMS` | Arm64 is cross-build verification only. |

Main compared all 152 build-manifest file hashes with the current source
`998325b` and found zero mismatches. The frozen candidate process on port
38482 used
`artifacts/project-setup-qa-20260910/run-20260910-213416/frozen-source/dev-control-room.exe`
with SHA-256
`2D46E73235896B68557B41E3868E712C28DA2A848D9539DF21B4E47B76D8CDC3`.

## Native Full source gate

The retained local evidence files
`artifacts/release-v0170-final-qa/verification-summary.json` and
`artifacts/release-v0170-final-qa/verification.log` record `mode: Full`,
`status: PASS`, and exit code 0 for the exact source SHA above. The environment
was native Windows with PowerShell 7.6.5, Go 1.26.7 windows/amd64, Node
v24.15.0, GCC 16.2.0, and Git 2.53.0.windows.1. These local artifacts are not
release assets.

| Summary gate name | Status |
| --- | --- |
| `gofmt-check` | **PASS** |
| `go-test` | **PASS** |
| `ui-syntax` | **PASS** |
| `go-test-race` | **PASS** |
| `go-vet` | **PASS** |
| `go-mod-verify` | **PASS** |
| `go-build` | **PASS** |
| `windows-amd64-build` | **PASS** |
| `windows-arm64-build` | **PASS** |
| `git-diff-check` | **PASS** |

Fast was not run separately. Full includes the Fast checks and is the recorded
source gate, so this is not blocking the candidate. The amd64 verification
binary hash is
`CF6BCB431A02EEFF785E7709CB260197CF3F6F15DD2FB1BD74E8DFA6C471D412`; it is a
direct verification-build hash, not a package SHA. The arm64 hash is recorded
in the summary as cross-build evidence only.

## Verification gates

| Gate | Status | Evidence required from main |
| --- | --- | --- |
| Focused M1 tests | **PASS via Full `go-test`** | The Full suite covers the candidate source; no duplicate focused rerun is required here. |
| Native Windows Fast verifier | **Not separately run** | Full is the superset source gate and passed. |
| Native Windows Full verifier | **PASS** | Exact record and environment are above. |
| Final browser acceptance | **PASS** | Seven routes, mixed fixture, filter/reload semantics, responsive geometry, screenshots, and console-error check. |
| Native OS folder-picker smoke | **NOT RUN** | Path-input discovery/registration was tested; the OS dialog was not. |
| Package/archive/hash smoke | **PENDING — not run** | Run `package.ps1`, inspect the ZIP, and record `SHA256SUMS`. |
| CI evidence | **PENDING** | Record the successor CI result for source `998325b`. |
| Tag/release/remote assets | **PENDING — not run** | User authorized publication; artifact checks and evidence remain required. |

## Final browser acceptance

The final browser gate used two distinct fixtures. The backend/frontend fixture
`project-setup-20260910` at SHA
`7b3d12905154dcc6edca8c1dd26696485df69cd1` covered Python/FastAPI and
JavaScript/Vue detection, and manual Python selection hid ESLint/Vitest actions
while retaining frontend evidence across reload. The `rootmix` fixture at SHA
`2e7f855ef905ae3177887d967713c311f632c8b7` contains no frontend; it covered
same-root Python and Go detection, real Go actions, manual Python hiding Go
actions, reload retention of the historical Go result, and reset after evidence
change. The rootmix fixture was registered through path entry, Find, and
register; blank-name autofill and asynchronous refresh into Work succeeded.

On `rootmix`, an ignored QA-only `ruff.toml` with line length 100 was detected.
Periodic refresh reset the manual preference to automatic, and the Korean
changed-HEAD/evidence notice plus Ruff configuration finding were displayed.
No Python/Vue analyzer ran. The UI coverage run
`run-d3f5666229e77016` (local artifact under
`artifacts/project-setup-qa-20260910/run-20260910-213416/isolated-home/artifacts/assurance/`)
succeeded at 100% (1/1 statements). This is `rootmix` fixture coverage only,
not main-repository coverage.

All seven routes (`home`, `projects`, `work`, `assurance`, `diagnostics`,
`activity`, `guide`) had exactly one visible `h1`. At CSS widths 1,094 and 485,
`document.scrollWidth <= document.clientWidth`; `errorlogs[]` was empty.
Guide slide 2 survived reload. The result link
`#assurance?run=run-d3f5666229e77016` placed the anchor at viewport top 92.34 px;
the sticky header was about 63 px high, and focus moved to `main-content`.
Desktop screenshots covered Work, result, and Guide; narrow screenshots covered
Work and Guide. The narrow result was checked but not screenshot-claimed. The
485 px run is the actual minimum viewport produced by the 390 px override; no
390 px claim is made.

## Scope boundaries and remaining release work

Python/FastAPI and Vue support remains read-only setup guidance: no analyzer or
test execution, installation, AI/provider launch, configuration write, source
upload, or generic shell execution is claimed. Existing root Go `vet`, test,
and coverage actions remain available. The 100% result above is fixture-only;
this record does not claim main-repository coverage or all E2E coverage.

The native OS folder picker was not tested. The source was pushed to
`origin/main` for CI, but CI has not yet been recorded. Package generation,
ZIP/hash smoke, tag creation, release assets, and publication remain pending;
no package, tag, or release publication was performed.
