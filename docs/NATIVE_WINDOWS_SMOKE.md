# Native Windows acceptance summary

Updated: 2026-08-23

## Accepted baseline

The final native Windows 11 acceptance was run from the separate NTFS checkout
at commit `3dbc90d6f6f2c14e8cce99b7d150fbd6b4feccd4` (also `origin/main`). The
environment was PowerShell 7.6.4, Go 1.26.7, Windows amd64, and `gcc` on PATH.

The following gates passed:

- `go test ./...`, including the linked Worktree proof regression;
- `CGO_ENABLED=1 go test -race ./...`;
- `go vet ./...`, `go build ./...`, and `go mod verify`;
- ProcessRunner, loopback UI, embedded Checkset flow, Worktree fail-closed,
  and Action approval-bypass checks;
- native `MessageBoxW` No/Cancel/Yes behavior (Cancel creates no approval;
  Yes creates only a digest-bound approval/audit; no Action process starts);
- Task Scheduler dry-run and status (`exists:false`, exit 0; no task mutation).

The detailed append-only Windows run log remains in the native checkout's
`docs/NATIVE_WINDOWS_SMOKE.md`. Earlier failed and unavailable runs in that
log are historical; the `3dbc90d` section is the authoritative final result.

## Scope boundary

This accepts Milestones 0–3, including 3A–3D and the W5 ceremony, on native
Windows. It does not implement or exercise real Action process execution,
target mutation, postchecks, Scheduler install/uninstall, configured release
or cleanup (Milestone 4), Guidance/Agent Handoff/MCP (Milestone 5), or
repeated-failure safeguards (Milestone 6). Those remain separate, typed slices.

The UI, CLI, and future MCP adapters must continue to call one application
service. Secrets remain masked, the Action Broker remains the only mutation
boundary, telemetry remains disabled, and no unreviewed dependency may be
added.
