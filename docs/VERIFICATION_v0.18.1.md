# v0.18.1 verification record

Status: **RELEASED — automated gates passed; manual product findings open**
Updated: 2026-09-13

This patch verifies the repository-quality and component-aware installation
hardening added after v0.18.0. The release target is Windows amd64 only.

## Candidate identity

| Field | Result |
| --- | --- |
| Reviewed implementation source | `v0.18.1` tag target |
| Version | `0.18.1` |
| Target | Windows 11 amd64 |
| Release assets | One ZIP plus `SHA256SUMS` |
| Linux/macOS/arm64 assets | Not produced |

## Automated verification

| Check | Status |
| --- | --- |
| `pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full` | **PASS** |
| `pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Fast` | **PASS** |
| `go vet ./...` | **PASS** |
| focused Action Broker race tests | **PASS** |
| quality inspection UI Node tests | **PASS — 11 tests** |
| Windows process-tree timeout regression | **PASS — 10/10 reruns** |
| captured-writer startup pause test | **PASS — normal 50/50, race 20/20; prompt absence, no wall-clock threshold** |
| related startup diagnostics tests | **PASS — normal 20/20, race 10/10** |
| Windows storage process serialization | **PASS — normal 20/20, race 10/10** |
| Windows amd64 package/archive/version/checksum smoke | **PASS** |

Packaged executable version: `0.18.1`

The tests cover actual `result + *exec.ExitError` behavior, unexpected pytest
exit codes, failed-test mapping, runnable-only improvement proposals, artifact
tampering, returned plan digests, target-switch response races, mixed
FastAPI/Vue component installation, repeated plans, audit recovery, legacy
plan rejection, lock release, Windows path identity normalization, and safe
pre-execution storage-lock retries. The captured-writer startup test checks
that the interactive pause prompt is absent rather than relying on a fixed
machine-speed wall-clock limit.

## Post-release manual product audit

Audit target: downloaded `v0.18.1` running at `http://127.0.0.1:38471` on
Windows 11. The selected real Worktree was
`C:\Users\knowgyu\workspace_window\dev-control-room`, branch `main`, HEAD
`86a58aa1204ec610004996ba2f2485d6ff0d4cbd`. The audit used the live browser
surface and did not treat the release-gate tests as proof of visual or
end-to-end product acceptance.

### User-flow result matrix

| Surface / flow | Result | Observation |
| --- | --- | --- |
| 개선 (Home) | **PASS with UX findings** | Selected Worktree and next action render; large unused space remains and the first actionable step is not dominant enough. |
| 프로젝트 | **PASS with UX findings** | Registration opens with focus on the first field; edit and unregister dialogs open and cancel safely. Nine repositories and 17 Worktrees are shown together, including fixtures, clones, and temporary agent Worktrees. |
| 작업 · configuration | **PASS with UX findings** | `dev-control-room` detects Go and exposes only evidence-backed Go checks. The advanced discovery/action/external selectors independently default to `backend`, which can send a user to the wrong target. |
| 작업 · Go vet | **PASS** | `go vet -mod=readonly ./...` completed successfully and the result was linked to the selected Worktree. |
| 작업 · test/coverage | **PARTIAL / TIMEOUT** | The timeout state is shown and the button recovers, but the generated coverage artifact remains presented as active evidence. |
| 검증 | **FAIL — visual/data findings** | The AI-candidate checkbox inherits the generic full-width input rule, collapsing its text column to 0px and producing vertical text plus a large empty card. No quality score exists before a plan is generated and approved, which is technically honest but not an adequate first-use explanation. |
| 진단 | **PASS with UX/state findings** | Environment and Provider refresh complete without browser console errors. The provider rows still have excessive empty space/status-column separation; the guidance selector initially shows `backend` while the global selected Worktree is `dev-control-room`. |
| 활동 | **PASS with UX findings** | The table renders, but repeated scheduled no-op events (“프로젝트 0개”) dominate the recent history and obscure meaningful activity. |
| 사용법 | **PASS** | Five guide slides advance/retreat correctly and the slide state is reflected in the URL. |
| filters / deep links | **PASS** | Assurance filters are reflected in the URL and restore their state on reload. |
| browser runtime | **PASS** | No console errors and no horizontal overflow were observed at the audit viewport. Full assistive-technology and multi-device acceptance was not claimed. |

### Findings requiring follow-up

| ID | Priority | Finding | Evidence / location | Required improvement |
| --- | --- | --- | --- | --- |
| UX-18.1-01 | **P1** | Assurance AI-candidate control is visibly broken. | `internal/app/ui/app.css:2226`; live computed checkbox width 532px and description width 0px. | Add a dedicated checkbox reset (`width:auto`, `min-height:0`, no text-input padding) and allow the label content to flex. Add a browser regression assertion for the computed layout. |
| UX-18.1-02 | **P1** | Failed coverage can leave an active artifact. | `internal/app/assurance.go:574`; live run was `시간 초과` while `run-3a9a884bff11c953.coverage.out` was `활성`. | Treat artifacts from timed-out/failed runs as partial or invalid, exclude them from evidence/score aggregation, and show the reason beside the artifact. |
| UX-18.1-03 | **P1** | Target selection is not a single source of truth. | `internal/app/ui/app.js:2390`, `4722`; Worktree target was `dev-control-room`, while discovery/action/external/guidance controls showed `backend`. | Centralize selected Worktree state, initialize every selector from it, and preserve it after discovery/refresh rerenders. |
| UX-18.1-04 | **P1** | Discovery rerender loses the chosen target. | Live test selected `dev-control-room` for “기존 점검 찾기”; after the request the selector returned to `backend` and no proposal was shown. | Preserve the selected target across `loadWorkData` and show an explicit zero-result state tied to the requested Worktree. |
| UX-18.1-05 | **P2** | Parent-folder registration exposes unrelated repositories and temporary Worktrees as one flat list. | Live project snapshot: 9 repositories, 17 Worktrees, including `dev-control-room-e2e-fixture`, v0.5 clones, and `dcr-pilot-*` paths. | Group by repository origin, classify fixture/clone/temporary Worktree, default to the active repository, and add a “다른 발견 저장소” disclosure/filter. |
| UX-18.1-06 | **P2** | Assurance is honest but not actionable before the first plan. | Live page shows empty plan/score/proposal/comparison panels and many `확인 불가` metrics. | Make the first action explicit, explain the plan → approve → run sequence in one compact callout, and hide non-actionable downstream metrics until evidence exists. |
| UX-18.1-07 | **P2** | Activity history is noisy. | Live page showed 30 rows, mostly repeated scheduled no-op scans. | Collapse identical no-op events, prioritize failed/changed/real-run events, and add date/type filters or pagination. |
| UX-18.1-08 | **P2** | Diagnostics provider layout still resembles a broken status rail. | Live screenshot: large row height, separated status dots, and sparse provider content. | Replace the status rail with compact status badges and a two/three-column provider row; keep details disclosure underneath. |
| UX-18.1-09 | **P3** | Dynamic selects and the AI checkbox have no consistent `name`/form identity. | Live control audit found unnamed dynamic selects and an unnamed checkbox; labels/ARIA labels were present. | Add stable names where controls participate in a form and keep labels, names, and URL state aligned. |

### Safety boundaries exercised

- Project registration, edit, unregister confirmation, and cancel paths were
  opened; no project or repository was deleted.
- Environment/Provider refresh, local Go vet, and read-only discovery were
  exercised. No external Jenkins, production, release, Scheduler, cleanup, or
  real package installation action was executed.
- The quality-tool installation surface was opened as a preview-only form; no
  tool installation was performed.

### Manual acceptance conclusion

`v0.18.1` is usable for the narrow, evidence-backed Go vet path and its basic
navigation/registration flows. It is **not fully accepted as a polished
repository-quality product** until UX-18.1-01 through UX-18.1-04 are fixed.
The release asset itself remains the verified Windows amd64 package; this
section records the product gaps discovered after running that release as a
real user would.

## Manual and external boundaries

- No real package installation is performed by the release gate.
- Jenkins, production systems, Scheduler mutation, and destructive cleanup are
  not contacted.
- Publication and downloaded-asset verification are performed after the tag is
  pushed and the GitHub release is published.
