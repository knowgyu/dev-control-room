# Normal-session workstream handoff

Updated: 2026-08-23

This is a coordination record for ordinary Codex native subagents. It is not an
OMX workflow state and must not activate or modify Ultragoal, Autopilot, or
Codex Goal artifacts.

## Current baseline

- Milestones 0–2, 3A, and 3B are accepted. 3C has a verified CLI/HTTP typed
  Checkset runner; its native Windows smoke remains pending.
- Slice 3D now includes the Action Broker, safe adapters, execution contract,
  and W5 trusted-human ceremony. It is not a generic shell and does not execute
  unreviewed repository text or any Action process.
- The approval ceremony is UI-only and empty-body-only. Windows opens a native
  `MessageBoxW` from persisted-plan data; non-Windows fails closed. CLI/API/MCP
  agents and schedulers cannot grant approval.
- Portability repairs are WSL/cross-verified only. Native Windows full
  tests/vet, interactive modal and UI smoke, Scheduler COM smoke, and race with
  `gcc` remain separate acceptance gates.

## Workstream ownership

| Stream | Owner model / role | Exclusive files | Permitted outcome | Must not do |
| --- | --- | --- | --- | --- |
| W1 — 3C UI parity | Luna xhigh executor | `internal/app/web.go`, UI-specific tests | Expose Checkset list/create/apply/run/result through existing application-service methods. | Duplicate policy, call Store or process runner directly, change Checkset domain/store contracts. |
| W2 — 3D Action core | Luna xhigh executor | `internal/action/**`, `internal/domain/model.go`, `internal/store/migrations.go`, `internal/store/repository.go`, Action-core tests | Typed plan/approval/lock/idempotency contracts and persistence only. | Add CLI/HTTP/UI adapters, mutate a target, or accept arbitrary shell input. |
| W3 — adapter integration | Luna xhigh executor, after W1/W2 acceptance | `internal/app/app.go`, `internal/app/service.go`, `cmd/dev-control-room/main.go`, Action HTTP routes | Thin application-service wiring and approval-bypass regressions. | Start until W1/W2 handoffs are accepted; own W1/W2 files. |
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
