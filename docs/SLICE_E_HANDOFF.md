# Slice E handoff: Action Broker core

Status: core, safe adapters, and the trusted-human approval ceremony are
implemented and accepted on WSL and native Windows at `3dbc90d`; Action
execution remains pending.

This slice adds no Action execution, target mutation, shell, connector, or
network surface. `internal/action.Broker` owns reviewed action-definition
resolution, plan persistence, trusted human-only approval construction,
digest-bound admission, holder-bound lock renewal, idempotency, and immutable
audit events. Every plan names a Project, Repository, and Worktree.

CLI and HTTP expose plan, read-only status, and admission through the
application service. They cannot grant approval: adapter-created requests are
stamped `system/adapter`, protected plans remain non-admissible, and no CLI,
API, MCP, agent, or scheduler route accepts an approval decision.

The only approval path is the protected, empty-body-only UI ceremony route.
It asks the Broker to derive the plan, digest, Worktree, executable, and expiry
from persisted state; the request cannot supply an actor, digest, expiry, or
decision. On Windows, the Broker opens a native `MessageBoxW` on the input
desktop. Yes grants a 15-minute, digest-bound `local-user` approval; No records
a rejection; Cancel records no decision. Non-Windows builds fail closed because
the native human prompt is unavailable. Only one ceremony may be active.

No adapter may access Store directly or execute an Action. No Action process,
shell, connector, target mutation, or postcheck runs in this slice. A future
execution slice must revalidate its exact target immediately before execution
and record postcheck evidence.

The execution-contract slice is also complete: an ActionPlan has a
digest-bound typed executable/argv/environment/timeout/evidence contract and
an immutable exact Worktree execution snapshot. Admission requires a current
`verified_read_only` Worktree plus a separately persisted matching execution
trust snapshot; changed, tombstoned, or untrusted Worktrees fail closed. This
does not launch a process or grant approval.

The portability repairs passed WSL tests, race tests, vet, module verification,
Linux build, Windows amd64/arm64 cross-builds, and `git diff --check`. Native
Windows full tests/vet/build/module/race, MessageBoxW behavior, loopback UI,
Worktree fail-closed, Action non-execution, and Scheduler dry-run/status also
passed. Scheduler install/uninstall and Action execution were not performed.
