# Roadmap

Status: active status index
Updated: 2026-08-27

The active detailed feature plan is Phase 1,
[AI_CODE_ASSURANCE_PLAN.md](AI_CODE_ASSURANCE_PLAN.md). Its follow-on Phase 2
product-usability work is separately planned in
[PHASE_2_PRODUCT_USABILITY_PLAN.md](PHASE_2_PRODUCT_USABILITY_PLAN.md).
Historical plans and verification documents remain evidence; they are not
instructions to rerun completed work.

| Area | Current status | Source of truth |
| --- | --- | --- |
| P0 local control plane | implemented and accepted | milestone verification documents and docs/HANDOFF.md |
| P1 configured project/release foundation | source implemented; native 0.5.0 UI/provider/cleanup/release acceptance has recorded gaps | docs/HANDOFF.md and docs/NATIVE_WINDOWS_SMOKE.md |
| P2 Agent Profiles, Handoff, MCP, safeguards | implemented foundation; managed assurance expansion is active | docs/AI_INTEGRATION.md and docs/AI_CODE_ASSURANCE_PLAN.md |
| AI-assisted Code Assurance | v0.10.0 release with full gate passed: durable Unattended Approval Scope lifecycle, Plan/Admit/Execute revalidation, fail-closed restart-boundary recovery, and effect-evidence separation/trace hardening are focused-verified; native process resilience, provider authority, mutation, patch adoption, and causal attribution remain partial | docs/AI_CODE_ASSURANCE_PLAN.md, docs/VERIFICATION_MILESTONE_A_APPROVAL_SCOPE.md, docs/ASSURANCE_EFFECT_DASHBOARD.md, and milestone verification documents |
| Product usability and interface | v0.10.2 adds a compact Home effect-proof summary linked to Assurance; v0.10.1 focus and empty-state polish remains; full Tab/Space, native dialog Esc, and second-device acceptance remain #7 | docs/PHASE_2_PRODUCT_USABILITY_PLAN.md and ../DESIGN.md |
| AI 작업 harness | user-scoped frontend, Vercel UI review, Go reliability, browser, screenshot, and CLI skills are installed; routing and evidence rules are active, with no runtime dependency | docs/AI_WORK_HARNESS.md |

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
- PowerShell 7.6 runbook plan surface and Action Broker typed execution boundary;
  configured native script acceptance remains.
- Local repository synchronization and fast-forward validation.
- Separate backend/frontend stage and production procedures.
- Opt-in read-only GitHub workflow and Jenkins build collectors are implemented;
  generic Jenkins target groups, queue correlation, and bounded partial-failure
  results are implemented; company-specific procedures remain runbook-owned.
- PR, branch, worktree, and explicitly linked issue cleanup queue.
- Explicitly approved linked-Worktree cleanup with fail-closed re-observation.
- Generic stage/production release preview and build-result postcondition
  evidence; expected-revision and provider-specific deployment evidence remain
  gated by a configured contract.
- Mechanical Guidance Doctor checks.

The former next implementation batch in docs/POST_MVP_EXECUTION_PLAN.md is
historical. Its generic v0.5.0 source scope covers grouped Jenkins work,
approved linked-worktree cleanup, and redacted release/post-deploy evidence;
the documented native acceptance gaps remain open. Company-specific targets,
credentials, production deployment, and release publication remain outside
automated fixture verification.

Outcome: common pre-PR, release, and cleanup work is fast, repeatable, and
auditable without asking an AI to rediscover commands.

## P2: AI assistance foundation

- Configurable Codex, Claude, Gemini, and `claude-local` Agent Profiles.
- Previewable, masked Agent Handoff and terminal launch.
- Stdio MCP adapter over the stable application service.
- Repeated-failure clustering and safeguard proposals.
- Shadow-mode safeguards, false-positive feedback, approval, and retirement.
- Optional semantic Guidance Doctor batch.

Outcome: humans and agents use the same deterministic control plane, while AI
is invoked only for bounded judgment-heavy work.

Managed non-interactive assurance sessions, provider/model selection, usage and
price-equivalent ledger, Quality Runs, artifacts, and the effect-proof screen are
implemented slices under docs/AI_CODE_ASSURANCE_PLAN.md. Patch adoption and
reverification authority remain human-owned.

## P3: optional operational visibility

- Read-only Kubernetes Pod status and bounded log retrieval are implemented
  through the configured REST API; workload/rollout and mutation operations
  remain deferred.
- Optional Harbor inspection.
- Compact reliability, rule-effectiveness, connector-freshness, and
  control-plane health metrics.

## Deferred or superseded

- Managed Hardener/QA-style assurance is superseded by the active
  AI_CODE_ASSURANCE_PLAN.md; generic multi-agent role orchestration remains
  deferred.
- CRAP as a product metric or required gate remains deferred.
- Multi-user RBAC and organization-wide deployment.
- Generic plugin marketplace or visual workflow builder.
- Automatic production deployment.
- Full knowledge base or raw-conversation ingestion.
- Team or individual productivity scoring.
