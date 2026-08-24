# Slice F handoff: Action execution contract

Status: execution contract, typed Action execution, and trusted-human approval
ceremony are implemented with WSL evidence on 2026-08-24. Native Windows
runtime acceptance remains pending for this new execution code.

Schema v11 persists exact Worktree execution-trust snapshots. Server-owned
Action definitions produce digest-bound typed executable identity, argv,
environment-name allowlist, timeout/output limits, and pre/postcheck evidence
contracts. Broker admission fails closed unless the persisted snapshot, current
Worktree observation, and explicit execution trust all match.

The Action executor launches only the persisted server-owned typed contract:
argv, allowlisted child environment, exact Worktree directory, timeout, bounded
output, and process-tree containment. It records masked `ActionRun` results and
pre/post evidence through the Broker. The approval surface remains a protected,
empty-body-only UI ceremony: on Windows it opens a native `MessageBoxW` from
server-derived persisted-plan metadata. CLI/API/MCP agents, schedulers, and
request bodies cannot grant an approval; non-Windows builds fail closed because
no native prompt is available.

WSL tests, race tests, vet, module verification, Linux build, Windows
amd64/arm64 cross-builds, and diff checks passed. Native Windows interactive
modal and the new Action process execution remain a separate acceptance gate;
Scheduler install/uninstall was not performed.
