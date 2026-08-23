# Slice F handoff: Action execution contract

Status: execution contract and trusted-human approval ceremony are implemented
and accepted on WSL and native Windows at `3dbc90d`; Action process execution
remains pending.

Schema v11 persists exact Worktree execution-trust snapshots. Server-owned
Action definitions produce digest-bound typed executable identity, argv,
environment-name allowlist, timeout/output limits, and pre/postcheck evidence
contracts. Broker admission fails closed unless the persisted snapshot, current
Worktree observation, and explicit execution trust all match.

No Action process, shell, connector, target mutation, or postcheck runs in this
slice. The approval surface is a protected, empty-body-only UI ceremony: on
Windows it opens a native `MessageBoxW` from server-derived persisted-plan
metadata. CLI/API/MCP agents, schedulers, and request bodies cannot grant an
approval; non-Windows builds fail closed because no native prompt is available.
Only a later execution owner may revalidate and launch a typed Action.

The portability repairs passed WSL tests, race tests, vet, module verification,
Linux build, Windows amd64/arm64 cross-builds, and diff checks. Native Windows
interactive-modal validation, full tests/vet/build/module/race, loopback UI,
Worktree fail-closed, and Scheduler dry-run/status also passed. No Action
process was launched; Scheduler install/uninstall was not performed.
