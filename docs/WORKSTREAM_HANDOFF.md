# Normal-session workstream handoff

Updated: 2026-08-23 (native Windows acceptance integrated)

This is a coordination record for ordinary Codex native subagents. It is not an
OMX workflow state and must not activate or modify Ultragoal, Autopilot, or
Codex Goal artifacts.

## Current baseline

- Milestones 0–3 (3A–3D) are accepted at `3dbc90d`; the native Windows final
  acceptance is summarized in `docs/NATIVE_WINDOWS_SMOKE.md`.
- Slice 3D now includes the Action Broker, safe adapters, execution contract,
  and W5 trusted-human ceremony. It is not a generic shell and does not execute
  unreviewed repository text or any Action process.
- The approval ceremony is UI-only and empty-body-only. Windows opens a native
  `MessageBoxW` from persisted-plan data; non-Windows fails closed. CLI/API/MCP
  agents and schedulers cannot grant approval.
- Portability repairs passed WSL/cross checks and native Windows full
  tests/vet/build/module/race, interactive modal/UI, Worktree fail-closed,
  Action non-execution, and Scheduler dry-run/status checks. Scheduler
  install/uninstall and Action execution remain intentionally unperformed.

## Next boundary

The next implementation is a separately typed Action execution slice. It must
revalidate the exact Worktree immediately before launch, keep argv and the
allowlisted environment separate from shell text, record bounded output and
postchecks, and remain behind the Action Broker. Only after that slice is
accepted should Milestone 4 configured release/cleanup work begin. Milestones
5 and 6 remain not started.

## Workstream ownership

| Stream | Owner model / role | Exclusive files | Permitted outcome | Must not do |
| --- | --- | --- | --- | --- |
| W1 — 3C UI parity | Luna xhigh executor | `internal/app/web.go`, UI-specific tests | Completed and accepted; preserve the shared application-service boundary. | Reopen accepted Slice C work without a regression. |
| W2 — 3D Action core | Luna xhigh executor | `internal/action/**`, `internal/domain/model.go`, `internal/store/migrations.go`, `internal/store/repository.go`, Action-core tests | Completed and accepted; planning/approval contracts only. | Add real Action execution or bypass the Broker. |
| W3 — adapter integration | Luna xhigh executor | `internal/app/app.go`, `internal/app/service.go`, `cmd/dev-control-room/main.go`, Action HTTP routes | Completed and accepted; thin application-service wiring and approval-bypass regressions. | Duplicate policy or introduce a second service boundary. |
| Vn — milestone verification | Terra high verifier/test engineer | Read-only by default | Verify the named completed stream against its acceptance contract; report exact evidence and gaps. | Edit product files, commit, push, or silently widen scope. |
| Final integration | Sol verifier | Clean integrated worktree | Assess architecture, security invariants, docs, tests, and release readiness. | Implement or waive an unresolved finding. |

Only the session leader stages, commits, and pushes. A task that needs another
stream's file stops and reports the requested handoff; it never takes the lock.

## Handoff contract

Each implementation stream ends with a short report containing:

1. exact files changed and the contract implemented;
2. tests/checks run, their output summary, and the toolchain/platform;
3. security invariants tested and any intentionally deferred behavior;
4. known gaps, requested next owner, and files that must remain locked; and
5. confirmation that no `.omx/ultragoal` state, secrets, telemetry, dependency,
   Action target, commit, or push was changed without its assigned authority.

The paired Terra verifier reads that report and the diff independently. A Sol
final verifier runs only after all stream-specific Terra verdicts pass and the
leader has one clean, integrated worktree.

## Common stop conditions

- Any secret reaches a persisted or presentation boundary.
- A route bypasses the application service or the Action Broker.
- A discovered string becomes executable without an explicit typed review
  boundary.
- A change needs an unreviewed dependency, real project path, credential, or
  external mutation.
- A stream needs a file owned by another active stream.

Native Windows 11 / PowerShell 7.6 smoke is a separate acceptance gate. WSL
tests and Windows cross-builds are useful evidence, not a replacement.
