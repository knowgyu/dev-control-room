# Slice F handoff: Action execution contract

Status: contract accepted in WSL; process execution and human approval are not
implemented.

Schema v11 persists exact Worktree execution-trust snapshots. Server-owned
Action definitions produce digest-bound typed executable identity, argv,
environment-name allowlist, timeout/output limits, and pre/postcheck evidence
contracts. Broker admission fails closed unless the persisted snapshot, current
Worktree observation, and explicit execution trust all match.

No Action process, shell, connector, target mutation, or human approval
surface exists in this slice. The next slice must establish a trusted human
authority separately from CLI, HTTP automation, MCP, agents, and schedulers;
only then may a later execution owner revalidate and launch a typed Action.

WSL tests, race tests, vet, module verification, Linux build, Windows amd64 and
arm64 cross-builds, and diff checks passed. Native Windows 11 / PowerShell 7.6
smoke remains unverified.
