# Native Windows acceptance summary

Updated: 2026-08-25

## Accepted baseline

The final native Windows 11 acceptance was run from the separate NTFS checkout
at product baseline commit `3dbc90d6f6f2c14e8cce99b7d150fbd6b4feccd4`. Later
release/docs commits preserved that product code and did not claim to rerun the
interactive native gate. The environment was PowerShell 7.6.4, Go 1.26.7,
Windows amd64, and `gcc` on PATH.

The following gates passed:

- `go test ./...`, including the linked Worktree proof regression;
- `CGO_ENABLED=1 go test -race ./...`;
- `go vet ./...`, `go build ./...`, and `go mod verify`;
- ProcessRunner, loopback UI, embedded Checkset flow, Worktree fail-closed,
  and Action approval-bypass checks;
- native `MessageBoxW` No/Cancel/Yes behavior (Cancel creates no approval;
  Yes creates only a digest-bound approval/audit; no Action process starts);
- Task Scheduler dry-run and status (`exists:false`, exit 0; no task mutation).

The required command gates all exited 0: `go test ./...`,
`CGO_ENABLED=1 go test -race ./...`, `go vet ./...`, `go build ./...`, and
`go mod verify`. The focused UI, Worktree, Action, ProcessRunner, and
Scheduler commands also exited 0. This summary is based on the operator's
native run; the raw append-only log is not present in this WSL checkout.

The detailed append-only Windows run log remains in the native checkout at
`C:\Users\knowgyu\workspace_window\dev-control-room\docs\NATIVE_WINDOWS_SMOKE.md`.
Earlier failed and unavailable runs in that log are historical; the `3dbc90d`
section is the authoritative final result.

## Scope boundary

This accepts Milestones 0–3, including 3A–3D and the W5 ceremony, on native
Windows. It does not exercise the new Action process execution, target
mutation, postchecks, Scheduler install/uninstall, configured release or
cleanup mutation, native Agent CLI launch, provider MCP clients, or safeguard
activation. The generic read-only Milestone 4–6 surfaces have separate WSL
evidence in their verification documents.

The UI, CLI, and MCP adapters must continue to call one application
service. Secrets remain masked, the Action Broker remains the only mutation
boundary, telemetry remains disabled, and no unreviewed dependency may be
added.

## Post-baseline Slice F gate

Typed Action execution and the related UI/API/CLI surfaces were added after
the accepted `3dbc90d` native run. They have WSL tests, race tests, vet,
module verification, and Windows cross-build evidence in
`docs/SLICE_F_VERIFICATION.md`, but are not covered by the native results above.
Repeat the native test and manual Action execution checklist from that document
before treating Slice F as natively accepted.

## Post-Milestone 5/6 operator gate

After the WSL suite passes, run the CLI examples in
`docs/AGENT_CLIENT_EXAMPLES.md` on Windows, verify Guidance Doctor against a
real selected Worktree, and exercise `devroom mcp serve` with a provider that
supports stdio MCP. Confirm that handoff preview remains non-launching, then
exercise the protected handoff launch with its returned preview digest. Confirm
the selected profile, exact Worktree, allowlisted environment, detached
process, and no transcript content. Review repeated-failure proposals as
shadow-only;
no activation or cleanup mutation is expected from this build.

## 2026-08-24 RC acceptance: stopped before manual gates

Timestamp: `2026-08-24T21:21:56+09:00`  
Requested baseline SHA: `64742d87d0dda53a5c149531589b72acf1cf18b3`  
Checked-out SHA: `64742d87d0dda53a5c149531589b72acf1cf18b3`

This was a native Windows 11 NTFS checkout run from PowerShell `7.6.4`, not
WSL. Tool versions were Go `go1.26.7 windows/amd64` and `gcc.exe (MSYS2) 16.2.0`.

| Command | Exit | Result |
| --- | ---: | --- |
| `go test -count=1 ./...` | 0 | PASS; all packages passed. |
| `CGO_ENABLED=1 go test -count=1 -race ./...` | 0 | PASS; all packages passed. |
| `go vet ./...` | 0 | PASS. |
| `go build ./...` | 0 | PASS. |
| `go mod verify` | 0 | PASS; all modules verified. |
| `go build -trimpath -o artifacts\\dev-control-room.exe .\\cmd\\dev-control-room` | 0 | PASS; SHA-256 `e572d9bf8f75c302c155673ea23783c61a29fde2c10c54a3505d2da34e5b158e`. |

The Slice F manual fixture setup did **not** satisfy the temporary-home
requirement: its PowerShell script attempted to assign the reserved `$HOME`
variable, so PowerShell retained the process default home. The fixture command
therefore opened the application with the default user home instead of the
intended fresh temporary application home. The generic fixture itself was
local-only and its Git scan returned `verified_read_only`, but this result is
invalid for acceptance.

To preserve existing user data, no default-home files are deleted here. No UI,
MCP, Guidance Doctor, handoff preview, Action execution, cleanup mutation,
production operation, Scheduler install, or Scheduler uninstall is claimed by
this run. The RC tag and prerelease were not created.

## 2026-08-24 Slice F RC acceptance: passed

Timestamp: `2026-08-24T21:42:29+09:00`  
Accepted source SHA: `0c41b126d817d54c5871dd053f86209596e074ba`

This was a native Windows 11 NTFS checkout using PowerShell `7.6.4`, Go
`go1.26.7 windows/amd64`, and `gcc.exe (MSYS2) 16.2.0`. The checked-in
`prepare-slice-f-acceptance.ps1` fixture returned an `appDataRoot` below a
fresh temporary `dev-control-room-rc-*` root; it did not use the default user
home or existing application data.

| Command | Exit | Result |
| --- | ---: | --- |
| `go test -count=1 ./...` | 0 | PASS. |
| `CGO_ENABLED=1 go test -count=1 -race ./...` | 0 | PASS. |
| `go vet ./...` | 0 | PASS. |
| `go build ./...` | 0 | PASS. |
| `go mod verify` | 0 | PASS; all modules verified. |
| `prepare-slice-f-acceptance.ps1 -BinaryPath artifacts\\dev-control-room.exe` | 0 | PASS; fixture source SHA matched the accepted source SHA. |

The native loopback UI showed Projects and repositories, Pre-PR checksets,
Environment Health, Actions, Cleanup queue, Guidance Doctor, and Repeated
failures. Environment Health displayed tool/profile findings. The cleanup
candidate was read-only and `blocked`, with every uncertainty represented as a
blocking reason. Repeated-failure safeguards remained shadow-only (no
proposal was activated).

In the selected generic fixture Worktree, the UI planned
`repository.refresh`, required an explicit execution-ready transition, then
ran the server-owned typed `git fetch --prune` against the fixture's local bare
origin. The reviewed ActionRun was `succeeded` with no output. No approval was
granted by CLI/API/MCP; the native approval ceremony remains a separate UI-only
boundary. The complete suite covers changed/unverified scope, expired approval,
lock, nonzero-exit, idempotency, timeout/process-tree, argv, and masking
fail-closed behavior.

Guidance Doctor checked only the selected fixture Worktree. Handoff preview
used the selected optional model metadata, reported `transcriptIncluded:false`,
and did not launch an agent. The typed stdio MCP `initialize`, `tools/list`,
and `project.list` calls succeeded. Its tool list exposed only `project.list`,
`finding.list`, `cleanup.list`, `guidance.check`, and `handoff.preview`; no
generic shell, unrestricted file reader, Action approval, or Action execution
tool was present.

The documented CLI examples were executed with the fixture Project,
Repository, and Worktree IDs. CLI/API/MCP/SQLite output was inspected through
the fixture flow; no secret value or canary was exposed. No production action,
destructive cleanup, Scheduler install, or Scheduler uninstall was performed.

The Windows amd64 artifact SHA-256 before release packaging was
`6b1211b0e1ce6d594e8d85712291556ee4d419070dda6ab989de7a4d3a191de6`.

### Release publication status

The native acceptance passed, but publication was not attempted: this Windows
operator environment has neither the GitHub CLI nor a configured GitHub API
token. To avoid credential discovery or a partial release, no `v0.4.0-rc.1`
tag, prerelease, asset upload, or downloaded-asset smoke test was created.
The fixture server was stopped by its exact fixture PID; its temporary evidence
root was intentionally preserved.

## 2026-08-24 v0.4.0-rc.2 corrected prerelease

The initial RC.2 asset was withdrawn after downloaded-asset smoke found that
its embedded version still reported RC.1. The corrected source updated the
CLI, MCP, and native fixture version contract to RC.2, then passed native
`go test -count=1 ./...`, `CGO_ENABLED=1 go test -count=1 -race ./...`,
`go vet ./...`, `go build ./...`, and `go mod verify`.

The corrected annotated tag `v0.4.0-rc.2` points to
`fb2e7c2`. Its prerelease asset `dev-control-room.exe` and `SHA256SUMS` are
published at <https://github.com/knowgyu/dev-control-room/releases/tag/v0.4.0-rc.2>.
The asset SHA-256 is
`965ee743fb9732994ef5112f360c9ce70fc23a3ded2e523c8770c7c8f74533e0`.

A fresh download reported `0.4.0-rc.2`; stdio MCP `initialize` and
`tools/list` also reported RC.2 and exposed only typed read-only tools. With
the default data directory and no command-line options, the downloaded binary
stayed running on `127.0.0.1:38471` and returned HTTP 200 from `/api/health`.
The prior apparent immediate exit was a local test process already holding
that port, not a released-binary failure. No production, destructive cleanup,
or Scheduler installation work was performed.

## 2026-08-24 RC publication: completed

The authenticated Linux publisher created annotated tag `v0.4.0-rc.1` at
acceptance-log commit `d4dda2f88de9d7b9866ab84220cc948e080b4c1a` and
published a GitHub prerelease:

<https://github.com/knowgyu/dev-control-room/releases/tag/v0.4.0-rc.1>

The Windows amd64 release asset was rebuilt with Go 1.26.7 from a clean clone
of the tag target using `GOOS=windows GOARCH=amd64 CGO_ENABLED=0` and
`-trimpath`. Embedded build metadata reports the exact tag-target revision and
`vcs.modified=false`. Its SHA-256 is
`4e4f76e2e973fb38b682f28a905944b8eaec30cf875a35413e35b1c1e7412686`.

Both release assets were downloaded again from GitHub into a fresh directory.
`sha256sum -c SHA256SUMS` passed; the downloaded PE32+ x86-64 executable
reported version `0.4.0-rc.1`, and its Windows stdio MCP smoke returned the
same server version and five typed tools. No production action, cleanup
mutation, or Scheduler mutation was performed during publication.

## Post-RC Korean UI usability acceptance: pending

The current source tip is `a870bb03ffc4b1991145f045f970d3081de312c3`.
The post-MVP batch must add a new native acceptance entry after its code and
docs commits; this file does not claim native acceptance for that batch.

Run this only after the implementation commit is available in a native Windows
11 NTFS checkout. Record the accepted source SHA, PowerShell/Go versions, built
EXE SHA-256, and exact result below this section. Use a fresh temporary
`appDataRoot` returned by `scripts/prepare-slice-f-acceptance.ps1`; do not assign
or override PowerShell's reserved `$HOME` variable and do not use the default
Dev Control Room application data.

Automated gates:

```powershell
go test -count=1 ./...
go test -count=1 -race ./...
go vet ./...
go build ./...
go mod verify
go build -trimpath -o artifacts\dev-control-room-ui.exe .\cmd\dev-control-room
Get-FileHash artifacts\dev-control-room-ui.exe -Algorithm SHA256
```

Manual UI acceptance on the fixture service:

1. Confirm the UI opens in Korean and the `홈`, `프로젝트`, `작업`, `진단`, and
   `기록` navigation shows one screen at a time. Check keyboard focus and a
   narrow browser width.
2. Confirm the Home screen prioritizes `지금 확인할 항목`, Project status, and
   recent execution results instead of raw logs.
3. In `진단`, confirm unavailable `codex`, `claude`, or `gemini` entries remain
   distinguishable as `Tool` versus `Agent Profile` rather than appearing as
   unexplained duplicates.
4. In Project detail, change the Project name, add a second temporary Git
   repository below the acceptance root with `새 저장소 등록`, then change its
   display name/path back to the exact fixture path. Export the Project, import
   the exported JSON into a fresh fixture home, and confirm no secret value is
   present in the file or UI.
5. Filter Findings by severity and lifecycle, open one evidence disclosure,
   verify confidence and first/last observed times, and mark one open Finding
   as `확인함`.
6. In `작업`, select the exact fixture Worktree and complete `기존 점검 찾기 →
   제안 근거 검토 → 제안 적용 → Checkset 만들기 → 적용 → 실행`. Confirm a
   rejected proposal cannot create a Checkset and a stale proposal is visibly
   distinct.
7. Expand Checkset and Action results. Confirm each result shows the exact
   Worktree/HEAD, status, exit code, masked stdout/stderr, and Action pre/post
   evidence. Confirm approval records and audit events are reviewable without
   implying that an approval ceremony itself executed the Action.
8. In `진단`, add and edit a temporary Agent Profile, verify its launch mode,
   data boundary, model template, and environment-name allowlist, then remove
   it. Preview a Handoff and confirm the scope, working directory, Findings,
   verification commands, masking state, and `전체 대화 기록: 포함하지 않음`
   are rendered as structured fields rather than raw JSON.
9. In Project detail, cancel Repository unregister once, enter a mismatched name
   once, then enter the exact Repository ID. Confirm only the final attempt
   sends the removal, the repository disappears from the registry, its files
   still exist, and the remaining last-Repository button is disabled with the
   Project-unregister explanation.
10. Cancel Project unregister once, then confirm it with the exact Project name.
   Confirm the Project disappears, both fixture directories still exist, and
   removal activity is visible in `기록`.
11. Recreate or retain a fixture as needed and smoke Pre-PR Checkset review,
   Action planning/trust/execution review, Guidance Doctor, Handoff preview,
   Cleanup candidates, and repeated-failure safeguards from their new screens.
12. Produce the same fixture Checkset or Action failure three times. In
    `진단 -> 반복된 실패`, confirm one persistent proposal appears with exact
    Project/Repository/Worktree scope and no raw output. Enter an owner and start
    shadow mode. Produce one different same-scope failure and one exact failure;
    confirm miss and hit each increase once. Record `유효한 방지`, activate the
    rule, roll it back to shadow, then retire it. Restart the service and confirm
    lifecycle, owner, local human activation approval, and metrics persist. Verify an unprotected request and an
    `/api/safeguards/{id}/activate` request cannot mutate the rule.

Do not perform production work, destructive cleanup, Scheduler
install/uninstall, release tagging, or release publication as part of this UI
acceptance. Temporary fixture data may be removed only after its exact path is
verified and the acceptance evidence has been copied into this document.

## Post-MVP Workstreams 2–4 acceptance: pending

The current post-MVP source adds protected HTTP/service contracts for grouped
Jenkins work, explicitly approved linked-Worktree cleanup, and generic stage or
production release evidence. No native Windows acceptance is claimed for these
contracts until the source commit below is checked out in a fresh NTFS copy.

Run the repeatable gate from PowerShell 7.6 in that checkout:

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full
```

Then run the manual checks below with a fresh temporary `appDataRoot` from
`scripts/prepare-slice-f-acceptance.ps1`; never assign or override `$HOME` and
never use the default application-data directory:

1. Confirm the five Korean screens and the Action result review checklist
   above, including exact Worktree/HEAD, masked output, prechecks, postchecks,
   and audit events.
2. Configure only a local fixture Jenkins HTTP server and fixture environment
   variables such as `JENKINS_USERNAME` and `JENKINS_TOKEN`. Verify a nested
   completed-build URL previews its Jenkins base path, folder job, selected
   `/build` or `/buildWithParameters` endpoint, credential references, and no
   credential value. Do not point the app at a company or production endpoint.
3. Plan a two-target external group, change one target or credential reference,
   and confirm the old plan is stale. With a fresh plan and trusted fixture
   Worktree, verify that approval is required, queue correlation returns the
   actual build number, and one failed target does not hide its sibling result.
4. Use a safe linked Worktree fixture only to preview cleanup. Verify dirty,
   untracked, detached, locked, prunable, ahead, missing-upstream, primary,
   and stale candidates remain blocked. Do not execute destructive cleanup on
   a user-important checkout.
5. Plan both `stage` and `production` release paths. Verify stage is an
   external-change plan, production is high-impact, both require explicit
   approval, successful-build evidence is recorded, and an expected revision
   without provider revision evidence is `postcheck_failed`.

Record the exact source SHA, OS, PowerShell, Go, Node, gcc, commands, exit
codes, fixture root, and any unavailable provider/UI gap below this section.
These checks do not authorize production deployment, release publication,
Scheduler mutation, real credentials, or cleanup outside the exact fixture.
