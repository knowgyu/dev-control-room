# Native Windows acceptance summary

Updated: 2026-08-23

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

The UI, CLI, and future MCP adapters must continue to call one application
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
supports stdio MCP. Confirm that handoff preview does not launch a process or
include transcript content. Review repeated-failure proposals as shadow-only;
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
