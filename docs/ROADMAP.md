# Roadmap

## P0: dependable local control plane

- Project with multiple repositories.
- Windows-native background service and Task Scheduler integration.
- Stable CLI with human and JSON output.
- Local Git and per-worktree branch, HEAD, upstream, dirty/untracked,
  detached/locked/prunable collectors, including linked `.worktrees` paths.
- Finding model with severity, confidence, evidence, and lifecycle.
- Environment, executable, and PowerShell-profile doctor.
- Project overview, finding detail, and evidence drill-down UI.
- SQLite event and current-state persistence.
- Failure-event collection and fingerprint fields from the beginning.

Outcome: opening the app in the morning shows what needs attention across
registered projects without reading raw logs.

## P1: first configured release project

- Read-only discovery of existing repository/CI automation for a selected
  Worktree, followed by proposal, review, and explicit apply.
- Approved pre-PR Checksets imported from existing documentation and verified
  against repository/CI configuration without inventing duplicate tooling.
- PowerShell 7.6 Action runner with prechecks, approvals, masking, and postchecks.
- Local repository synchronization and fast-forward validation.
- Separate backend/frontend stage and production procedures.
- Opt-in GitHub and Jenkins collectors and Jenkins triggers.
- PR, branch, worktree, and explicitly linked issue cleanup queue.
- Mechanical Guidance Doctor checks.

Outcome: common pre-PR, release, and cleanup work is fast, repeatable, and
auditable without asking an AI to rediscover commands.

## P2: AI assistance without orchestration

- Configurable Codex, Claude, Gemini, and `claude-local` Agent Profiles.
- Previewable, masked Agent Handoff and terminal launch.
- Stdio MCP adapter over the stable application service.
- Repeated-failure clustering and safeguard proposals.
- Shadow-mode safeguards, false-positive feedback, approval, and retirement.
- Optional semantic Guidance Doctor batch.

Outcome: humans and agents use the same deterministic control plane, while AI
is invoked only for bounded judgment-heavy work.

## P3: optional operational visibility

- Read-only Kubernetes Dashboard/pod status and bounded log retrieval after the
  actual API/version is verified.
- Optional Harbor inspection.
- Compact reliability, rule-effectiveness, connector-freshness, and
  control-plane health metrics.

## Deferred

- Managed agent runs and multi-agent role orchestration.
- Specifier/Cleaner/Hardener/QA/CRAP workflow products.
- Multi-user RBAC and organization-wide deployment.
- Generic plugin marketplace or visual workflow builder.
- Automatic production deployment.
- Full knowledge base or raw-conversation ingestion.
- Team or individual productivity scoring.
