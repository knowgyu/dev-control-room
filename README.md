# Dev Control Room

Dev Control Room is a Windows-native, local-first engineering control plane for
one developer. It continuously diagnoses explicitly registered projects,
presents evidence-backed findings, and runs reviewed deterministic operations.

The background service is the product. The browser UI is its control room, and
the CLI and optional MCP adapter expose the same capabilities to coding agents.
Codex, Claude, Gemini, and `claude-local` remain optional workers rather than the
source of truth.

## What it should solve

- Group several repositories under one project, such as a generic
  `sample-project` composed of multiple repositories.
- Show actionable findings by severity instead of making the user read logs.
- Diagnose Git, worktrees, pull requests, CI, local tools, PowerShell profiles,
  and environment configuration on a schedule.
- Run reviewed pre-PR checks and PowerShell release procedures without an AI
  round trip.
- Keep production and destructive actions behind explicit approval.
- Hand a bounded failure bundle to a user-selected coding agent when judgment
  or code changes are needed.
- Turn repeated verified failures into proposed deterministic safeguards.

## Product contract

- Windows 11 and PowerShell 7.6 are the target runtime; WSL is not required.
- Only explicitly registered projects, paths, and connectors are observed.
- The service binds to loopback and sends no telemetry.
- Secrets never appear in logs, events, CLI JSON, HTTP responses, MCP results,
  prompts, or agent handoff bundles.
- GitHub is detected as a repository capability, not modeled as a separate
  top-level workspace.
- Human UI, CLI, and MCP all use the same application service and policy engine.
- AI is never required for known checks or runbooks.
- Dependencies require an approved permissive license and network review.

## Repository status

Milestones 0–3 are implemented and accepted, including SQLite-backed Agent
Profile CRUD, Environment Doctor, Worktree-aware Git observation, deterministic
discovery proposals, typed read-only Checksets, and the Action Broker's
planning/approval boundary. Native Windows acceptance passed at `3dbc90d`.

Real Action process execution, target mutation, postchecks, Scheduler
install/uninstall, configured release/cleanup (Milestone 4), Guidance/Agent
Handoff/MCP (Milestone 5), and repeated-failure safeguards (Milestone 6) have
not started. The former JSONL ledger and in-memory snapshot spike adapters are
no longer used.

The target stack is a small Go service and CLI, an embedded local web UI,
SQLite-backed state, PowerShell 7.6 runbooks, and Windows Task Scheduler for
startup and catch-up scheduling. A stdio MCP adapter is planned after the stable
CLI/application contracts exist.

## Start here

- [Current state and implementation handoff](docs/HANDOFF.md)
- [Product definition](docs/PRODUCT.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Configuration and secrets](docs/CONFIGURATION.md)
- [AI and agent integration](docs/AI_INTEGRATION.md)
- [Implementation plan](docs/IMPLEMENTATION_PLAN.md)
- [Luna implementation brief](docs/LUNA_IMPLEMENTATION_BRIEF.md)
- [Roadmap](docs/ROADMAP.md)
