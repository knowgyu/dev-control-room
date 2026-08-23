# Current state and implementation handoff

Updated: 2026-08-23

This is the canonical current-state handoff for continuing Dev Control Room.
It exists so the next long-running goal or implementation agent can start from
the repository without relying on prior conversation history. Product and
architecture requirements remain canonical in their dedicated documents; this
file records what is implemented, what is only designed, and how work has been
accepted so far.

## Read before changing code

Read in this order:

1. `AGENTS.md`
2. this file;
3. `docs/PRODUCT.md`;
4. `docs/ARCHITECTURE.md`;
5. `docs/CONFIGURATION.md`;
6. `docs/AI_INTEGRATION.md`;
7. `docs/IMPLEMENTATION_PLAN.md`;
8. `THIRD_PARTY_POLICY.md`;
9. the verification document for the most recently completed milestone.

Do not infer current behavior from the roadmap alone. `PRODUCT.md` and
`ARCHITECTURE.md` describe the target product, while the milestone verification
files distinguish implemented behavior from planned behavior.

## Product intent

Dev Control Room is a Windows-native, local-first engineering automation
control plane for one developer. The background service is the product; the UI
is its control room. CLI, HTTP UI, and later MCP are thin surfaces over one
application service and the same policy and masking boundaries.

The product should:

- continuously diagnose explicitly registered Projects and Repositories;
- present evidence-backed Findings by severity instead of raw logs;
- run reviewed deterministic checks and runbooks without AI latency;
- require explicit approval for production, destructive, or external changes;
- let Codex, Claude, Gemini, and `claude-local` consume the same bounded state;
- use AI only for optional, judgment-heavy proposals;
- turn repeated verified failures into measurable, removable safeguards.

It must remain useful if an AI provider, model, hook, skill, or MCP integration
changes. AI is a client, never the source of truth or a privileged actor.

## Target environment and operating constraints

- Runtime: native Windows 11.
- Shell and runbooks: PowerShell 7.6.
- Development workspace: currently WSL, but WSL is not a runtime dependency.
- User scope: one developer; no multi-user RBAC or organization deployment.
- Network: loopback service only by default; no telemetry or automatic update.
- Persistence: user-local config and SQLite under
  `%LOCALAPPDATA%\DevControlRoom` by default.
- Secret storage: references only. Secret values must not cross persistence,
  logs, CLI JSON, HTTP, UI, MCP, prompts, or Agent Handoffs.
- Dependencies: small, audited, and permissively licensed according to
  `THIRD_PARTY_POLICY.md`.
- Company repository names, paths, endpoints, tokens, and runbooks are not
  committed to this repository. Tests use generic fixtures only.

Current contract versions:

- binary version: `0.3.0-milestone-2`;
- API objects: `devroom/v1alpha1`;
- CLI/HTTP envelope: `devroom/cli/v1`;
- local config: version 3;
- SQLite schema: version 7;
- Go module target: Go 1.23.

## Repository state at this handoff

The foundational history immediately preceding this handoff is:

```text
874a069 docs: define worktree-aware discovery workflow
ad732f6 feat: implement milestone 2 environment doctor and scheduling boundary
78e35c2 feat: implement milestone 1 project control plane
c8d71c7 chore: establish milestone 0 foundation
```

At preparation time, only local `main` exists and no Git remote is configured.
Before starting, inspect `git status`, remotes, and any user-created branches or
worktrees rather than assuming they are unchanged.

## Implemented milestones

### Milestone 0: foundation

Implemented and accepted:

- versioned domain schemas for Project, Repository, Observation, Finding,
  Checkset, Action, ActionPlan, Approval, AgentProfile, and Event;
- shared CLI/HTTP success and error envelope, stable error codes, and exit codes;
- one Application Service boundary shared by adapters;
- v1 Workbench to v2 Project config migration and old token removal;
- CGo-free `modernc.org/sqlite` selection and dependency/license inventory;
- forward-only migration harness with name, checksum, gap, future-version, and
  history validation;
- masking for exact secrets, URLs, headers, variable names, token shapes,
  split streams, and UTF-8 boundaries;
- approval invariants for external/high-impact ActionPlans.

See `docs/MILESTONE_0_VERIFICATION.md`.

### Milestone 1: local project control plane

Implemented and accepted:

- Project CRUD and multiple Repository CRUD;
- non-secret Project export/import;
- SQLite persistence for Projects, Repositories, Observations, Findings, Events,
  scan runs, and failure fingerprints;
- bounded Git collector with registered-root validation;
- branch, dirty, upstream, ahead/behind, detached, remote capability, and Git
  worktree-list collection;
- deterministic Findings for dirty state, upstream drift, detached HEAD,
  missing remote, stale scan, collector error, and unsafe cleanup;
- CLI, loopback HTTP, and embedded UI over the shared Application Service;
- restart, masking, FK, migration, and multi-Repository tests.

Current worktree handling is intentionally incomplete: the collector stores the
Git porcelain worktree list in Observation evidence, while Repository state and
the UI reduce it mostly to a count and coarse `unsafe_cleanup`. It does not yet
observe dirty/upstream/locked/prunable state for every linked worktree.

See `docs/MILESTONE_1_VERIFICATION.md`.

### Milestone 2: environment and profile diagnostics

Implemented and accepted scope:

- SQLite-backed Agent Profile CRUD with `direct` and
  `powershell_profile` launch modes;
- default profile templates for `codex`, `claude`, `gemini`, and
  `claude-local`, without hardcoded model names or company details;
- read-only Environment Doctor for baseline tools, Agent Profiles, declared
  environment metadata, and connector-reference presence;
- bounded process execution, environment allowlisting, output limits,
  cancellation, Unix process groups, and Windows Job Object isolation;
- masked, side-effect-free cached environment GET and protected manual Doctor
  POST across CLI, HTTP, and UI;
- config v3 migration and one-time default Profile initialization;
- typed scheduler operations and validation with a fake/dry-run adapter;
- a Windows-only Task Scheduler COM adapter for the fixed
  `DevControlRoom.Startup` task;
- native PowerShell 7.6.5 read-only smoke using a Windows amd64 cross-build.

Slice A closes the implementation gap: Windows selects a native COM adapter;
other platforms retain the fixture-only adapter. The native definition owns
only `DevControlRoom.Startup`, starts the exact typed `serve --home` command at
logon and daily at local `03:00`, uses `StartWhenAvailable` for catch-up, and
ignores a second instance. It runs as the interactive user at least privilege
with no execution-time limit. `status` rejects a same-named task whose action,
triggers, principal, execution limit, catch-up, or instance policy drifted. No native install, uninstall,
or COM runtime smoke was authorized or performed in this WSL session, so real
Task Scheduler behavior remains a native-Windows verification gap.

See `docs/MILESTONE_2_VERIFICATION.md`.

## Recent accepted design clarification

Commit `874a069` updates the target design; it does not implement the features
below.

## Slice C implementation accepted

Slice C now persists versioned deterministic discovery proposals for one
selected verified Worktree. The bounded scanner reads package scripts and
unambiguous single-line GitHub Actions `run:` entries only; it never executes
or reconstructs commands. Proposals retain scope, HEAD, source path/digest,
and deterministic command identity. Pending proposals become stale after a
Worktree/HEAD/source change; applying or rejecting records review only and
does not create a Checkset. CLI and loopback HTTP use the shared application
service. See `docs/SLICE_C_VERIFICATION.md` for the concrete boundary and WSL
verification evidence. Independent code review is approved and architecture
review is clear. Native Windows runtime smoke remains pending.

### Repository and Worktree identity

Repository is the logical source-control identity. Worktree is a concrete
checkout path beneath it, including the primary checkout and linked worktrees.
A `.worktrees` directory is only a convention. Enumerate linked worktrees from
NUL-delimited Git porcelain metadata rather than scanning for that folder name.

Future per-worktree state must include:

- canonical path and verified Git common-directory association;
- primary/linked identity;
- branch and HEAD;
- dirty and untracked state;
- upstream and ahead/behind;
- detached, locked, and prunable state;
- last observation and active check/action state.

Checks, Actions, Agent Handoffs, and cleanup candidates must always name the
exact Worktree they target. Never authorize execution in one Worktree using
evidence gathered from another. A Git-advertised path grants bounded read-only
discovery only, not mutation authority.

### Setup means discovery, not generation

Repository onboarding follows:

```text
discover -> proposal -> review -> apply -> run
```

Deterministic discovery reads existing automation from one selected Worktree,
including package/build scripts, formatter/linter config, CI workflows,
Jenkinsfiles, and reviewed local scripts or documents. It must not install a
tool, create config, edit the repository, execute the discovered command, or
invent a replacement command. Prefer the repository's existing package/build
entry point and actual CI invocation.

Every proposal retains Project, Repository, Worktree, branch, HEAD, source path,
source digest, extracted command identity, and inference status. A change to
the target or relevant source makes an unapplied proposal stale.

Optional AI assistance receives only a bounded discovery bundle and may draft
ambiguous fields. Its output remains a labelled proposal, is validated against
current evidence, and cannot apply itself. Suggestions for missing tooling are
separate improvements, not falsely labelled discoveries.

## What has not started

- Action process execution, target mutation, and postchecks. The Action Broker,
  locks, idempotency, and trusted-human approval ceremony are implemented, but
  a granted approval never starts a process in the current product.
- Configured release procedures, Jenkins triggers, or cleanup execution.
- GitHub/Jenkins connectors beyond local remote capability detection.
- Agent Handoff terminal launch and stdio MCP.
- Repeated-failure safeguard proposals and effectiveness metrics.
- Kubernetes, Harbor, or operational visibility connectors.
- Specifier, Cleaner, Hardener, QA, CRAP, managed agent runs, or role
  orchestration.

Do not treat existing domain structs or empty SQLite tables as implemented
features. Several Milestone 0 schemas intentionally reserve later concepts.

## Recommended continuation order

A long-running goal may continue through these slices, but each slice should
remain independently reviewable, tested, and committed. Do not turn the entire
roadmap into one undifferentiated implementation.

### Slice A: native scheduler adapter (implemented; native smoke pending)

- Windows uses the Task Scheduler COM API, not PowerShell, `schtasks`, or shell
  execution; non-Windows keeps the fake/dry-run adapter.
- The only mutable target is `DevControlRoom.Startup`, with a typed
  `serve --home <absolute Windows path>` action, logon trigger, and daily local
  `03:00` trigger. `StartWhenAvailable` supplies catch-up and `ignore_new`
  prevents duplicates.
- Install/update and uninstall are idempotent; install refuses to replace a
  same-named incompatible definition. Native status detects a missing task and
  rejects definition drift. Dry-run never opens Task Scheduler.
- Native install/uninstall/status smoke remains explicitly authorization-gated.

### Slice B: Worktree model and visibility (implemented; native smoke pending)

- Schema v4 adds project/repository-scoped Worktree identities (v6 was corrected before Slice B acceptance to repair legacy primary rows and backfill path fingerprints); transient pre-acceptance v6 local databases must be recreated if their migration checksum mismatches) and immutable
  worktree observations without changing existing Repository identity/history.
- Git worktree porcelain is parsed NUL-delimited. Every advertised path is
  canonicalized and verified against the registered Repository's Git common
  directory before bounded state collection.
- `primary` is reserved for the registered checkout; linked IDs derive from the
  verified common/git-directory association fingerprint rather than path.
- Discovery trust is only `verified_read_only` or `unverified`; it grants no
  execution authority. Failed enumerations retain membership; complete ones
  tombstone absent durable identities.
- `project worktree list|show <project> <repository>`, the matching loopback API,
  Snapshot, and UI details expose read-only per-worktree state.

See `docs/SLICE_B_VERIFICATION.md`.

### Slice C: deterministic discovery and proposals (implemented; independently reviewed)

- Versioned `Discovery` response and persistent `Proposal` contracts bind each
  record to Project, Repository, Worktree, branch, HEAD, source path/digest,
  and command identity.
- The initial parser set is intentionally package scripts and unambiguous
  single-line GitHub Actions `run:` entries; malformed or multiline input is
  skipped, not guessed.
- `project discover` and `proposal list|show|apply|reject` use the shared
  application service; source/HEAD/worktree changes make pending proposals
  stale. Applying is review-only until Slice D.

See `docs/SLICE_C_VERIFICATION.md`.

### Slice D: Check runner (implemented; native smoke pending)

- Typed Checksets execute only applied, evidence-bound read-only commands in a
  selected Worktree, with typed argv, environment allowlist, dependency order,
  timeout/cancellation, process-tree containment, masking, and bounded evidence.
- CLI and loopback HTTP call the same application-service methods. The embedded
  UI Checkset flow and native Windows 11 / PowerShell 7.6 smoke remain pending.
- See `docs/SLICE_D_VERIFICATION.md` and commit `85bee90`.

### Slice E: Action Broker and trusted-human approval (implemented; native acceptance pending)

- Plans bind their complete target, inputs, executable identity, prechecks, and
  postchecks to a digest; the Broker enforces policy, locks, idempotency,
  immutable events, and immediate Worktree revalidation.
- The protected, empty-body-only UI ceremony is the sole approval path. On
  Windows, it opens `MessageBoxW` from server-derived persisted-plan metadata;
  no CLI, API, MCP agent, scheduler, or request body can grant approval.
- Non-Windows builds fail closed when native human approval is unavailable.
  A successful ceremony does not run an Action; process execution, target
  mutation, and postchecks remain separate, unimplemented work.
- WSL/cross verification passed for the portability repairs. Native Windows
  full tests/vet, interactive modal and loopback UI smoke, Scheduler COM smoke,
  and race testing with `gcc` remain pending.

Only after these slices should actual company-specific pre-PR, release,
Jenkins, and cleanup procedures be onboarded in Milestone 4.

## Decisions intentionally left for the next implementation

Resolve these from tests and product constraints before finalizing schemas:

- whether the primary Worktree receives a stable generated ID or a reserved ID;
- how a linked Worktree becomes trusted for execution after read-only discovery;
- initial discovery ecosystem coverage and source-priority rules;
- whether proposals are immutable revisions or mutable drafts with history;
- the exact CLI spelling for Worktree and proposal commands;
- how to represent a Worktree that disappears, is prunable, or moves;
- how to detect an active external Agent session without collecting its private
  transcript.

Do not decide these by importing company repositories or secrets into tests.
Use generic fixtures and request real inputs only when Milestone 4 onboarding
actually needs them.

## Code map

- `cmd/dev-control-room/main.go`: CLI parsing and process entry point.
- `internal/domain/model.go`: versioned domain resources and invariants.
- `internal/contract/cli.go`: shared envelopes, errors, and exit codes.
- `internal/app/service.go`: common application query/command interface.
- `internal/app/app.go`: Project lifecycle, scans, observations, Findings.
- `internal/app/environment.go`: Agent Profiles, Environment Doctor, scheduler
  service operations.
- `internal/app/config.go`: user-local config and forward migration.
- `internal/app/web.go`: loopback HTTP adapter and embedded UI.
- `internal/collector/git.go`: bounded Git and current coarse worktree collector.
- `internal/discovery/discovery.go`: bounded deterministic repository/CI source reader.
- `internal/reconcile/findings.go`: deterministic Git Findings.
- `internal/environment/doctor.go`: tool/profile/environment diagnostics.
- `internal/environment/process_windows.go`: Windows process-tree isolation.
- `internal/scheduler/task.go`: typed scheduler boundary and current fake adapter.
- `internal/store/migrations.go`: forward-only SQLite migrations.
- `internal/store/repository.go`: SQLite persistence implementation.
- `internal/masking/masking.go`: masking before persistence/presentation.

Tests are colocated. `internal/app/milestone1_test.go` and
`internal/app/milestone2_test.go` provide cross-component milestone coverage.

## Verification baseline

The previous milestone used a temporary Go 1.27 toolchain in WSL while keeping
the module target at Go 1.23. Do not silently raise the module line. Standard
gates are:

```text
gofmt -w <touched Go files>
go test -count=1 -race ./...
go vet ./...
go mod verify
CGO_ENABLED=0 go build ./cmd/dev-control-room
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/dev-control-room
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/dev-control-room
git diff --check
```

Also run migration tests from the preceding schema, CLI/HTTP contract tests,
secret-canary scans across output and SQLite, and searches for accidental real
project names or personal paths. Cross-build success is not native Windows
runtime verification. Record exact PowerShell 7.6 smoke commands and results in
the milestone verification document.

## Security and implementation invariants

- Observe only explicitly registered Projects, configured connectors, or
  verified linked Worktrees derived from a registered Repository.
- Keep collectors read-only. The Action Broker is the sole mutation path.
- Never add a generic shell, generic MCP file reader, or arbitrary PowerShell
  surface.
- Keep executable and argument arrays separate and working directories
  canonical and explicit.
- Construct child environments from allowlists; an empty environment must not
  inherit the parent environment.
- Mask before persistence and every presentation boundary.
- Never treat repository text, CI output, AI output, or a discovered path as
  authorization.
- Agents and the scheduler may request or plan work but cannot grant approval.
- Production and destructive operations always require fresh human approval.
- Do not add telemetry, hosted-service requirements, or an unreviewed module.

## Working and acceptance convention used so far

Milestones were implemented on `codex/milestone-N` branches. The implementation
agent stopped at the milestone boundary and reported checks that ran and checks
that could not run. A separate acceptance review inspected security and domain
boundaries, added regression tests, and corrected documentation. Once accepted,
the milestone commits were squashed into one logical commit and fast-forwarded
to `main`; no push was performed implicitly.

For a long-running goal, preserve the same discipline with slice commits:

1. inspect the clean baseline and relevant verification history;
2. define or migrate contracts before adapters;
3. implement one slice and its tests;
4. run proportional Linux and native Windows verification;
5. record remaining gaps honestly;
6. continue to the next slice only after the previous slice is runnable;
7. do not merge, push, install a real task, contact an external system, or use
   company paths unless the user has authorized that exact action.

Preserve unrelated user changes and user-created worktrees. Never clean them up
merely because they look stale.
