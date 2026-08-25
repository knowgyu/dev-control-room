# Implementation plan

Status: historical foundation plan
Superseded as an active queue: 2026-08-26

Milestones 0–6 in this document were implemented and their acceptance evidence
is retained in the milestone verification documents. Use
docs/AI_CODE_ASSURANCE_PLAN.md for the active implementation sequence; do not
rerun these milestones merely because they remain described here.

This is the execution plan for an implementation agent. Complete milestones in
order and keep every milestone runnable on Windows 11 with PowerShell 7.6.

## Milestone 0: contract and spike reconciliation

Goal: turn the current exploratory code into a safe foundation before adding
features.

Work:

- Treat existing `Workbench`, in-memory snapshot, JSONL ledger, and embedded
  HTML as a spike, not a compatibility contract.
- Define versioned Project, Repository, Observation, Finding, Checkset, Action,
  ActionPlan, Approval, AgentProfile, and Event schemas.
- Define stable CLI JSON envelopes, error codes, and process exit codes.
- Define application query/command interfaces shared by CLI and HTTP.
- Choose and document the SQLite driver after license, telemetry, binary, and
  Windows compatibility review; record it in `THIRD_PARTY_POLICY.md`.
- Add config migration and database migration test harnesses.
- Add a masking/redaction package and tests before persisting command output.
- Replace terminology from Workbench to Project in domain-facing code.

Exit criteria:

- Architecture decisions are represented in code interfaces and tests.
- `go test ./...`, `go vet ./...`, and Windows `go build` pass.
- No production action, connector, MCP server, or AI workflow is added yet.

## Milestone 1: Project, CLI, persistence, and local Doctor

Goal: provide a useful AI- and human-readable local control plane.

Work:

- Add/remove/list Projects and add/remove/list multiple Repositories.
- Store configuration under `%LOCALAPPDATA%\DevControlRoom`; support export and
  import of non-secret Project configuration.
- Implement SQLite repositories for observations, current findings, events,
  scan runs, and failure fingerprints.
- Implement local Git and worktree collectors using bounded process execution.
- Detect remote-provider capability from normalized Git remotes.
- Implement deterministic reconcilers for dirty state, upstream drift,
  detached HEAD, missing remotes, stale scan, and unsafe cleanup conditions.
- Implement `devroom project`, `devroom finding`, `devroom event`, and
  `devroom serve` commands with `--json`.
- Rebuild the UI around Project cards and severity Findings; keep raw logs as
  drill-down evidence.

Exit criteria:

- A sample Project can contain three Windows repository paths.
- Scheduled and manual scans produce the same normalized findings.
- CLI JSON is covered by golden/schema tests.
- Restarting the service retains Projects, history, and finding lifecycle.

## Milestone 2: environment/profile Doctor and Windows scheduling

Goal: diagnose the user's actual Windows development environment safely.

Work:

- Resolve and version-check `pwsh`, `git`, `gh`, `codex`, `claude`, `gemini`,
  and the profile-backed `claude-local` command.
- Inventory declared environment-variable names and scopes without persisting
  values.
- Detect missing, conflicting, duplicated, and stale declarations.
- Add connector-auth health checks that return masked summaries.
- Add `devroom env doctor` and the Environment Health UI.
- Install/uninstall/query the Windows Task Scheduler entry via explicit CLI
  commands; implement daily catch-up behavior.
- Test native Windows paths, process trees, cancellation, output limits, and
  PowerShell profile modes from PowerShell 7.6.

Exit criteria:

- The Doctor explains whether every configured Agent Profile and connector is
  available without revealing a token.
- The service starts at login and does not run duplicate scheduled instances.

## Milestone 3: Checksets and Action broker

Goal: run known checks and local operations without AI latency.

Work:

- Introduce a Worktree identity beneath Repository for the primary checkout and
  every linked Git worktree. Record canonical path, branch, HEAD, dirty and
  untracked state, upstream drift, detached/locked/prunable state, and last
  observation. A `.worktrees` directory is a convention, not a special case;
  discovery comes from `git worktree list --porcelain -z`.
- Show expandable per-worktree status in UI and structured CLI output. A
  Repository summary may retain counts, but it must not hide the worktree that
  a check, Action, cleanup candidate, or Agent Handoff targets.
- Add deterministic repository discovery for existing `package.json` scripts,
  formatter/linter configs, Make/Task/Just files, language build metadata,
  CI workflows, Jenkinsfiles, and reviewed local scripts or documents.
- Make discovery read-only. It may not install a tool, create configuration,
  edit a repository, or invent a replacement command. Prefer an existing CI or
  package/build entry point over reconstructing its underlying command.
- Persist discoveries as proposals with source evidence, selected worktree,
  branch, HEAD, and relevant file digests. Require explicit review before
  applying a proposal, and mark it stale when its target or source changes.
- Allow optional AI assistance only for ambiguous proposal drafting. AI output
  is labelled as inference, constrained to the discovered evidence, validated
  against the same schema, and cannot apply or activate itself.
- Implement typed executable/argument/working-directory/environment definitions.
- Implement Checkset execution, cancellation, timeout, status normalization,
  masking, and evidence capture.
- Implement Action planning separately from Action execution.
- Implement risk policy, approval records, execution locks, postchecks, and
  idempotency keys.
- Add `devroom check` and `devroom action` CLI families.
- Add UI flows for pre-PR checks, plans, approval, execution, and result review.
- Import the user's existing pre-PR documentation as proposals and require
  review against the selected worktree's actual CI/repository configuration.
- Add discovery/proposal CLI and UI flows following
  `discover -> proposal -> review -> apply`; `run` executes only an applied
  Checkset. Every execution binds to one explicit worktree identity.

Exit criteria:

- A pre-PR Checkset can report passed, failed, skipped, and unavailable steps.
- The same Checkset behaves identically from UI and CLI.
- A linked worktree under `.worktrees` is visible with its own state, and a
  Checkset cannot silently run in a different worktree.
- Discovery of an existing repository creates no file and installs no tool;
  every applied step retains evidence of the existing command it uses.
- Secret-canary tests prove that stdout/stderr/event/API/CLI surfaces are masked.
- A caller cannot bypass required approval through HTTP or CLI.

## Milestone 4: configured release project and cleanup

Goal: support the first high-value project-specific operations.

Work:

- Onboard user-selected repository paths and a confirmed branch topology.
- Convert reviewed Obsidian PowerShell procedures into version-controlled
  scripts or typed Action definitions.
- Add local sync, fast-forward prechecks, backend/frontend separation,
  stage/production promotion, Jenkins triggers, and post-deploy evidence.
- Add configured GitHub/Jenkins read-only collectors. GitHub latest workflow
  and Jenkins latest build lookup are implemented; mutation remains pending.
- Implement cleanup candidate correlation for merged PRs, branches, worktrees,
  remote branches, and explicit issue links.
- Re-observe every candidate worktree immediately before cleanup and block
  dirty, untracked, detached, locked, active, missing-upstream, or unpushed
  states unless the reviewed policy explicitly proves the operation safe.
- Keep dirty/unpushed cleanup blocked and production/destructive operations
  always human-approved.

Exit criteria:

- Dry-run plans show the exact commits, branches, Jenkins job, inputs, and
  postchecks before execution.
- Production cannot run without a fresh explicit approval.
- Cleanup never removes a dirty worktree or unpushed commit in automated tests.

## Milestone 5: guidance, Agent Handoff, and MCP

Goal: let both humans and coding agents use the control plane comfortably.

Work:

- Add mechanical Guidance Doctor checks.
- Add configurable Agent Profiles and user-selected model metadata.
- Build previewable, masked handoff bundles and launch selected CLIs in the
  chosen Windows working directory.
- Keep full agent transcript collection off by default.
- Add a stdio MCP adapter with narrow typed tools over the application service.
- Apply identical project scope, masking, and Action-broker rules to MCP.
- Document thin hook/skill examples that call stable CLI commands; do not
  implement role orchestration.

Exit criteria:

- Codex, Claude, Gemini, and `claude-local` can consume structured project and
  finding data through CLI; MCP is verified for providers that support it.
- An MCP caller cannot access secrets or self-approve a protected Action.
- `Ask Agent` passes only the previewed bounded context.

## Milestone 6: repeated-failure learning

Goal: convert recurring mistakes into measured safeguards.

Work:

- Normalize CI, Checkset, hook, Agent Handoff verification, and Action failures.
- Cluster fingerprints using deterministic keys before optional AI analysis.
- Let the configured AI profile propose a safeguard with evidence and scope.
- Add shadow execution, feedback, false-positive metrics, approval, activation,
  rollback, and retirement.
- Add Guidance Doctor semantic proposals behind the same review boundary.

Exit criteria:

- A repeated failure can become a reviewed deterministic rule without editing a
  permanent Agent prompt.
- Every active rule reports hits, misses, false positives, cost, and owner.

## Cross-cutting verification

Every milestone must include:

- unit tests for domain and policy behavior;
- integration tests for process execution, persistence, and masking;
- Windows PowerShell 7.6 smoke commands;
- migration tests from the previous schema;
- dependency license and network-behavior review;
- evidence that no unrelated path, repository, or secret was observed;
- documentation updates when a contract changes.

Do not begin Kubernetes mutation, Harbor, a plugin framework, or role-based
Agent workflows until the relevant milestone above is accepted. The bounded
read-only Kubernetes status/log surface is an integration diagnostic and is
documented separately in `docs/INTEGRATIONS.md`.
