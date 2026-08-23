# Slice E handoff: Action Broker core

Status: core accepted in WSL; adapter and execution slices remain unstarted.

This slice adds no Action execution, target mutation, CLI, HTTP, UI, shell, or
network surface. `internal/action.Broker` owns reviewed action-definition
resolution, plan persistence, trusted human-only approval construction,
digest-bound admission, holder-bound lock renewal, idempotency, and immutable
audit events. Every plan names a Project, Repository, and Worktree.

The next adapter stream may call only `Plan`, `GrantHumanApproval`, `Admit`,
`Renew`, and `Release` through the application service. It must not construct
plans or approvals, access Store directly, or execute an Action. A future
execution slice must revalidate its exact target immediately before execution
and record postcheck evidence.

WSL tests, race tests, vet, module verification, Linux build, Windows amd64 and
arm64 cross-builds, and `git diff --check` passed. Native Windows 11 /
PowerShell 7.6 smoke remains unverified.
