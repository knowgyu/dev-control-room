# Product definition

## One-sentence definition

Dev Control Room continuously observes selected software projects, diagnoses
drift and failures, performs approved deterministic actions, and converts
repeated failures into reviewable safeguards.

## Primary jobs

1. Tell the user what needs attention across active projects each morning.
2. Explain every finding with current evidence, confidence, and a recommended
   next action.
3. Run known pre-PR, cleanup, and release procedures quickly and verify them.
4. Preserve operational continuity across Codex, Claude, Gemini,
   `claude-local`, terminals, and sessions.
5. Escalate bounded problems to a user-selected agent without turning the app
   into an agent orchestrator.
6. Learn from repeated verified failures without growing permanent prompt files.

## Core concepts

### Project

A Project is the top-level unit selected by the user. It contains one or more
repositories and optional capabilities. For example, a generic
`sample-project` can contain three repositories plus Jenkins, stage/production
release topology, and later a read-only Kubernetes connector. A small personal
project may enable local Git diagnosis only.

Unsupported or disabled capabilities are hidden from that project's UI.

### Repository

A Repository belongs to exactly one Project in the local configuration. Its Git
remote identifies optional provider capabilities such as GitHub PR and CI
status. GitHub is not a separate top-level object.

### Observation and Finding

An Observation is immutable evidence collected from Git, a process, a file, a
connector, or an action. A Finding is a current condition derived from one or
more observations.

Every Finding contains:

- stable type and fingerprint;
- project and optional repository scope;
- severity: `info`, `attention`, `high`, or `critical`;
- confidence: `confirmed`, `likely`, or `uncertain`;
- concise summary and recommended next action;
- evidence references and first/last observed timestamps;
- lifecycle state: open, acknowledged, resolved, suppressed, or expired.

Finding severity is distinct from Action risk.

### Action

An Action is a typed, deterministic operation with validated inputs, prechecks,
an approval policy, timeout, postchecks, masking, and an evidence record.

Action risk classes are:

- `read_only`: may run automatically;
- `safe_local`: may run automatically when project policy permits;
- `external_change`: requires confirmation by default;
- `high_impact`: always requires explicit human confirmation.

Production promotion, production Jenkins triggers, destructive cleanup, and
credential changes are always `high_impact` regardless of project overrides.
An `ActionPlan` for an external or high-impact action must require approval;
high-impact grants must be linked to the exact plan digest, issued through the
human approval surface, and carry a future expiry. Because this is a single-user
product, the human may approve an action they requested. Agents and the
scheduler can request or plan work but can never grant approval.

### Checkset

A Checkset is a reviewed collection of deterministic checks, such as a
repository's pre-PR checks. Results are `passed`, `failed`, `skipped`, or
`unavailable`; a local pass must never imply that unavailable remote CI checks
passed.

### Agent profile and handoff

An Agent Profile describes how to open `codex`, `claude`, `gemini`, or
`claude-local`, its optional model selection, data boundary, working-directory
behavior, and environment allowlist.

An Agent Handoff is a user-initiated, bounded package of masked findings,
evidence, relevant diffs, and verification commands. It is not a managed agent
run and does not own the agent's full lifecycle.

## Required product surfaces

### Morning overview

- project cards with counts by severity;
- stale or failed collectors;
- environment and tool health;
- release readiness for configured projects;
- cleanup candidates and pending approvals;
- last successful and next scheduled diagnosis.

Logs are drill-down evidence, not the primary interface.

### Environment and profile doctor

- resolve `pwsh`, `git`, `gh`, `codex`, `claude`, `gemini`, and
  `claude-local` without executing arbitrary profile code where avoidable;
- report versions and command-source conflicts;
- inspect configured user, machine, process, project, and PowerShell-profile
  environment sources;
- compare required variable names with actual presence and expected shape;
- test opt-in connector authentication without recording secret values;
- report missing, duplicate, conflicting, stale, or unverified configuration.

### Pre-PR checks

- import existing human-written CI/pre-PR documentation once;
- compare it with actual repository and CI configuration;
- save the user-approved result as a structured Checkset;
- run it from UI or CLI with the same behavior and machine-readable result;
- offer an Agent Handoff only after presenting deterministic evidence.

### Release operations

For projects that enable release capabilities:

- run reviewed PowerShell 7.6 scripts with argument arrays, not interpolated
  shell strings;
- update local repositories and verify clean/expected state;
- verify fast-forward eligibility before stage or production promotion;
- support separate backend and frontend procedures;
- trigger configured Jenkins stage or production jobs;
- perform explicit prechecks, approvals, postchecks, and evidence capture;
- never infer a production topology and act without user confirmation.

### Cleanup queue

- correlate merged PRs with local branches, remote branches, worktrees, and
  explicitly linked issues;
- never delete a dirty worktree or a branch containing unpushed commits;
- never close an issue based only on an AI inference;
- group candidates by project and explain why each is safe or blocked.

### Guidance doctor

Diagnose `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, runbook documents, and selected
knowledge files for missing references, invalid commands, duplication,
conflicts, excessive size, and stale metadata. Semantic stale-content analysis
may use a scheduled AI review, but changes are always presented as a diff for
human approval.

### Repeated-failure learning

Normalize CI, check, hook, and action failures into fingerprints. After repeated
verified occurrences, AI may propose a deterministic safeguard. A safeguard
must run in shadow mode, collect false-positive evidence, and receive human
approval before activation. It must remain measurable and removable.

## Product boundaries

Dev Control Room is not:

- a general knowledge base or raw-conversation archive;
- a replacement for GitHub, Jenkins, Kubernetes Dashboard, or an IDE;
- a chat-first coding agent or multi-agent role orchestrator;
- an automatic issue closer or production deployer;
- a team productivity or individual performance scoring system;
- a generic workflow marketplace.

Specifier, Cleaner, Hardener, QA, and CRAP-style agent workflows are deferred
agent-side extensions. The core may expose deterministic checks they can call,
but it does not orchestrate those roles in the initial product.
