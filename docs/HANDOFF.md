# Current state and implementation handoff

Updated: 2026-08-27

## 2026-08-27 AI 작업 harness

The current Codex desktop workspace now has a user-scoped, non-runtime skill
harness for the next implementation slices. It routes UI work through
`frontend-design`, Vercel `web-design-guidelines`, and browser/screenshot
verification; Go work through style, error, testing, concurrency, database,
troubleshooting, security, and observability guidance; and CLI work through
`cli-creator`. The exact workflow and effect-evidence chain are in
[AI_WORK_HARNESS.md](AI_WORK_HARNESS.md). These skills do not weaken the
Action Broker, masking, Worktree trust, npm Codex launcher boundary, or human
patch-adoption rule, and they are not runtime dependencies of the binary.

## 2026-08-27 Milestone A approval-scope slice

The durable Unattended Approval Scope slice is implemented on the current
working branch. A Scope is draft/approved/revoked, revisioned with SQLite CAS,
bound to a current observed Worktree, and matched against exact action,
Provider/tool, path, network, disk, deadline, and mandatory-prohibition fields.
The Broker records digest/match evidence at Plan and rechecks it at Admit and
Execute. Revocation or expiry fails closed and a pre-launch rejection releases
an acquired Action lease. Protected API routes cover list/get/check/create,
approve, and revoke. Read
[VERIFICATION_MILESTONE_A_APPROVAL_SCOPE.md](VERIFICATION_MILESTONE_A_APPROVAL_SCOPE.md)
and [ADR-003](decisions/ADR-003-unattended-approval-scope.md) before changing
this boundary.

## 2026-08-27 effect-evidence hardening slice

The Assurance impact surface now treats `measured`, `prevented_regression`,
`user_estimated`, `ai_inference`, and `unavailable` as separate classifications.
Measured time saved and user-estimated time saved are separate metrics; the
latter never becomes a measured benefit. Trace-ID-only source links contribute
to provider/model attribution, and an effect is counted as verified only when
its source and live artifact evidence are linked, its adoption and
reverification metadata point to the same commit, and the referenced run or
invocation succeeded at that exact HEAD. Trace drill-down includes source and
reverification execution nodes plus adopted/reverified commit links. Focused
tests cover these boundaries; the full verification record is
[VERIFICATION_v0.10.0.md](VERIFICATION_v0.10.0.md).

## 2026-08-27 v0.10.0 release candidate

The candidate combines the fail-closed Assurance restart-boundary recovery
with the effect-evidence hardening above. The binary version is `0.10.0`.
The candidate full verifier passed; Windows package smoke and release
publication are performed from the final documentation commit. Remaining
native, human-adoption, causal, and manual-accessibility gaps stay partial in
the plan. See [VERIFICATION_v0.10.0.md](VERIFICATION_v0.10.0.md) for the
candidate evidence and the final generated summary asset for the release
commit.

## 2026-08-27 v0.9.1 released package

The current candidate keeps the v0.9.0 evidence dashboard and adds actionable
startup diagnostics for double-click launches. Human CLI errors now show the
reason, action, and next command; `troubleshoot` checks the local home and
reads a safe `troubleshooting/latest.json` classification without exposing
paths, SQL, credentials, or raw provider output. JSON envelopes remain
machine-readable.

Same-home server, CLI, and direct migration access now share a companion file
lock with bounded retry. Context cancellation/deadline is distinct from a
genuinely busy store, and cross-process initialization/read/write tests pass.
The full package and remote hash verification is recorded in
[VERIFICATION_v0.9.1.md](VERIFICATION_v0.9.1.md), and the release is published
at https://github.com/knowgyu/dev-control-room/releases/tag/v0.9.1.

This is the canonical current-state handoff for continuing Dev Control Room.
It exists so the next long-running goal or implementation agent can start from
the repository without relying on prior conversation history. Product and
architecture requirements remain canonical in their dedicated documents; this
file records what is implemented, what is only designed, and how work has been
accepted so far.

## 2026-08-26 v0.9.0 effect-proof work

The working candidate adds the effect-proof surface described in
[ASSURANCE_EFFECT_DASHBOARD.md](ASSURANCE_EFFECT_DASHBOARD.md). The embedded
검증 screen now shows a selected period and project/provider/model scope,
previous-equal-period comparisons, daily/weekly trend, measurement boundaries,
traceability score, artifact storage, and trace drill-down. JSON/CSV report
downloads contain safe artifact references and never local storage paths.

Artifact results now carry a manifest version, storage key, masking-policy digest,
Trace ID, retention metadata, and archive manifest hash. Export uses a staging
directory and atomic rename; restore checks both the archive manifest and the
artifact SHA-256. Active/pinned retention changes verify the source bytes, and
the 512 MiB local quota rejects new results rather than silently dropping them.

The remaining boundary is intentional: this work records adoption and
reverification metadata but does not materialize patches or let an AI commit,
push, or claim causal productivity impact. Native Windows runtime and full
keyboard/dialog acceptance remain separate evidence items.

## Historical 2026-08-26 v0.8.0 release handoff

The v0.8.0 release preserved the earlier release history and added the bounded
real Codex path, registered local Quality runners, GitHub baseline fail-closed
handling, an Assurance-value dashboard, first-use/return/Provider-recovery
journeys, and Korean copy/accessibility refinements. Read
[VERIFICATION_v0.8.0.md](VERIFICATION_v0.8.0.md) for its exact commands and
release evidence; the v0.9.0 section above is the current result.

Verified at the v0.8.0 source candidate:

- `scripts/verify-real-codex.ps1`: real local authenticated Codex acceptance,
  46 assertions, typed `node.exe` plus verified `@openai/codex/bin/codex.js`,
  no shell launcher/raw transcript, clean fixture, and prompt non-persistence.
- `scripts/verify-phase2-journeys.ps1`: 239 assertions in fresh temporary
  homes and local Git fixtures, including first-use/return/recovery, local npm
  launcher, registered runners, blocked paths, and secret canary checks.
- Browser evidence: desktop/narrow first-use and return views, registration,
  finding deep-link focus, Provider recovery focus, project-card Enter, and
  dialog description/cancel handling. Console errors were empty.

This is not a claim that every Phase 1 exit criterion is complete. Native
Codex resilience is [#3](https://github.com/knowgyu/dev-control-room/issues/3),
provider-authoritative CI is [#4](https://github.com/knowgyu/dev-control-room/issues/4),
mutation execution is [#5](https://github.com/knowgyu/dev-control-room/issues/5),
complete accessibility/manual acceptance is [#7](https://github.com/knowgyu/dev-control-room/issues/7),
and the remaining persistence/patch/artifact/trend scope is
[#9](https://github.com/knowgyu/dev-control-room/issues/9)–[#12](https://github.com/knowgyu/dev-control-room/issues/12).

## Read before changing code

Read in this order:

1. `AGENTS.md`
2. this file;
3. `docs/VERIFICATION_PLAYBOOK.md`;
4. `docs/INTEGRATIONS.md`;
5. `docs/PRODUCT.md`;
6. `docs/ARCHITECTURE.md`;
7. `docs/CONFIGURATION.md`;
8. `docs/AI_INTEGRATION.md`;
9. `docs/AI_CODE_ASSURANCE_PLAN.md`;
10. `docs/decisions/ADR-001-ai-assisted-code-assurance.md`;
11. `docs/PHASE_2_PRODUCT_USABILITY_PLAN.md` (planned follow-on; read before
    any Phase 2 work, not as a reason to skip Phase 1);
12. `docs/IMPLEMENTATION_PLAN.md` (historical foundation only);
13. `THIRD_PARTY_POLICY.md`;
14. the verification document for the most recently completed milestone.

Do not infer current behavior from the roadmap alone. `PRODUCT.md` and
`ARCHITECTURE.md` describe the target product, while the milestone verification
files distinguish implemented behavior from planned behavior.

## Historical 2026-08-25 0.5.0 source checkpoint

The current source checkpoint is the pushed `main` tip. The earlier
Agent-Handoff continuation note below is historical and is retained only as
evidence; it does not describe the current working tree.

- The 0.5.0 source includes the grouped execution implementation from
  `60f599e`, per-target audit events from `643c972`, and the Korean UI flow for
  setup, external groups, release plans, and cleanup plans. Verify the exact
  checked-out and remote tip with `git rev-parse HEAD` and
  `git ls-remote origin refs/heads/main` before resuming.
- The only current untracked paths are user-created `.agents/` and
  `skills-lock.json`; neither may be staged or modified.
- The source includes the Korean five-screen UI, typed Action execution,
  repository-sync grouping, read-only GitHub/Jenkins/Kubernetes integration
  checks, PowerShell runbook planning, and safeguard lifecycle foundations.
- Generic grouped Jenkins work, explicitly approved linked-Worktree cleanup,
  and stage/production release plans with bounded successful-build evidence are
  implemented through protected HTTP and application-service paths. Expected
  revision evidence, provider-specific deployment contracts, and
  organization-specific PowerShell fallback behavior remain gated by local
  configuration and separate contracts.
- No native Windows acceptance is claimed for the 0.5.0 UI and grouped
  external/release/cleanup surfaces until the user runs the pending checklist
  in `docs/NATIVE_WINDOWS_SMOKE.md`. WSL, Windows toolchain, and native runtime
  evidence remain separate.

The prior continuation checkpoint beginning with `e536297` described a
different historical working tree and is superseded by this section.

## Historical 2026-08-25 Agent Handoff checkpoint

The historical checkpoint recorded digest-bound Handoff preview and launch
work, but it was not the current source baseline. Its exact validation notes
remain useful only as historical evidence.
- Repeatable native toolchain checks are documented in
  `docs/VERIFICATION_PLAYBOOK.md` and automated by
  `scripts/verify.ps1 -Mode Full`. The script does not replace the native UI,
  provider MCP, or configured-agent manual checklist in
  `docs/NATIVE_WINDOWS_SMOKE.md`.

Resume commands:

```text
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full
git status --short --branch
git diff --check
node --check internal/app/ui/app.js
go test -count=1 ./...
CGO_ENABLED=1 go test -count=1 -race ./...
go vet ./...
go mod verify
go build ./...
```

For native Windows, run the commands from the NTFS checkout and then follow
the current Handoff checklist in `docs/NATIVE_WINDOWS_SMOKE.md`. Treat the
preview digest as review evidence: preview again if the Worktree, profile,
model, or Findings change.

## Historical repository group sync checkpoint

The first generic multi-repository operation is now implemented without
company-specific configuration. A selected Project acts as a one-to-many
logical group for a latest-sync plan. The service, protected HTTP routes,
embedded Korean UI, and CLI all share the same persisted per-repository Action
plans and Broker execution boundary.

- Plan route: `POST /api/projects/{projectID}/repository-sync/plan`.
- Execute route: `POST /api/projects/{projectID}/repository-sync/execute` with
  only returned `planIds` and a caller request ID.
- CLI: `project sync plan` and `project sync execute`.
- Command: typed `git pull --ff-only --prune`; no shell interpolation.
- Eligibility: primary active Worktree, verified observation, clean and
  tracked branch, upstream present, no local-ahead commits, and not locked or
  prunable.
- Execution: bounded two-target concurrency, per-target ActionRun evidence,
  post-sync scan, and immediate Korean result summary with output details.
- Focused native Windows evidence: `go test -count=1 ./...`,
  `CGO_ENABLED=1 go test -count=1 -race ./...`, `go vet ./...`, `go build ./...`,
  and `go mod verify` all passed in an NTFS temporary checkout using Windows Go.

The first provider-specific operation is implemented as a read-only GitHub
latest workflow lookup. It resolves generic `owner`, `repository`, and
`workflow` values at request time through a protected UI route and keeps the
credential reference boundary intact. Jenkins latest build lookup is also
implemented as a read-only operation with nested Job path resolution and
Basic Auth or bearer credential-reference support. Kubernetes read-only Pod
status and bounded logs are implemented with namespace/selector resolution;
Jenkins triggers and Kubernetes mutation remain unimplemented. PowerShell
runbook CRUD and Broker-backed typed plan creation are implemented; a real
configured script still requires the separate native Windows smoke checklist.
No real identifiers or credentials belong in this repository.

## Product intent

Dev Control Room is a Windows-native, local-first engineering automation
control plane for one developer. The background service is the product; the UI
is its control room. CLI, HTTP UI, and MCP are thin surfaces over one
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

- binary version: `0.9.0`;
- API objects: `devroom/v1alpha1`;
- CLI/HTTP envelope: `devroom/cli/v1`;
- local config: version 3;
- SQLite schema: version 14;
- Go module target: Go 1.23.

## Repository state at this handoff

The foundational history immediately preceding this handoff is:

```text
874a069 docs: define worktree-aware discovery workflow
ad732f6 feat: implement milestone 2 environment doctor and scheduling boundary
78e35c2 feat: implement milestone 1 project control plane
c8d71c7 chore: establish milestone 0 foundation
```

The original native-accepted product baseline is
`3dbc90d6f6f2c14e8cce99b7d150fbd6b4feccd4`. Slice F and the generic
Milestone 4-6 surfaces were accepted on native Windows at
`0c41b126d817d54c5871dd053f86209596e074ba`; the acceptance log commit
`d4dda2f88de9d7b9866ab84220cc948e080b4c1a` is tagged and published as the
`v0.4.0-rc.1` prerelease. Inspect status, remotes, and user-created
branches/worktrees again before changing code.

Both native gates used Windows 11, PowerShell 7.6.4, Go 1.26.7, and `gcc`.
See `docs/NATIVE_WINDOWS_SMOKE.md`.

The post-RC Korean UI usability refresh is implemented in the current source
but has not yet received native Windows acceptance. The embedded UI now uses
five hash-routed screens (`홈`, `프로젝트`, `작업`, `진단`, `기록`) instead of
one long page, keeps Findings and Project cards primary, and exposes guarded
Project/Repository `등록 해제` controls over the existing protected DELETE
routes. Registration removal requires exact-name confirmation, states that
repository files are not deleted, and disables individual removal of the last
Repository. Environment findings retain separate `Tool` and `Agent Profile`
source labels, closing the presentation ambiguity tracked in GitHub issue #1.
See `DESIGN.md` and the pending checklist in `docs/NATIVE_WINDOWS_SMOKE.md`.

The final portability/acceptance sequence is `27b5aa3` (trusted-human
ceremony), `b8ab439` (Scheduler status), `b7b2491` (linked Worktree fixture),
and `3dbc90d` (canonical Worktree path on proof failure). Earlier verification
documents retain the evidence and limitations known at their original dates;
the later native result supersedes only their “pending native smoke” status.

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
triggers, principal, execution limit, catch-up, or instance policy drifted. The
native acceptance run verified scheduler dry-run/status without installing or
uninstalling a real task.

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
verification evidence. Independent code review is approved, architecture
review is clear, and the native Windows acceptance is recorded in
`docs/NATIVE_WINDOWS_SMOKE.md`.

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

- Company-specific release mutation and provider deployment evidence. Generic
  stage/production Jenkins group plans, queue correlation, bounded target
  results, and successful-build postchecks are implemented; expected revision
  and organization-specific behavior remain gated by configured contracts.
- Explicitly approved linked-Worktree cleanup is implemented through the Broker;
  remote branch deletion and company-specific cleanup policy remain out of
  scope.
- GitHub trigger/release operations and Kubernetes mutation remain unimplemented.
- Native configured PowerShell runbook acceptance and project-specific release
  procedure mapping.
- Native Agent Handoff launch and provider-specific MCP client acceptance.
- Hook and launched-Handoff verification failure producers and their safeguard
  normalization remain pending. Repository CI gates are present, but native
  UI/provider acceptance is still manual. Handoff launch metadata is not a
  verification result and deliberately does not create failure learning.
- Optional AI clustering beyond deterministic exact-fingerprint safeguards.
- Kubernetes mutation, Harbor, or operational visibility beyond the bounded
  Pod status/log surface.
- Generic multi-agent role orchestration and CRAP as a required metric remain
  unimplemented. The former managed Hardener/QA scope is superseded by the
  active plan in docs/AI_CODE_ASSURANCE_PLAN.md.

Do not treat existing domain structs or empty SQLite tables as implemented
features. Several Milestone 0 schemas intentionally reserve later concepts.

## Recommended continuation order

A long-running goal may continue through these slices, but each slice should
remain independently reviewable, tested, and committed. Do not turn the entire
roadmap into one undifferentiated implementation.

### Slice A: native scheduler adapter (implemented and natively accepted)

- Windows uses the Task Scheduler COM API, not PowerShell, `schtasks`, or shell
  execution; non-Windows keeps the fake/dry-run adapter.
- The only mutable target is `DevControlRoom.Startup`, with a typed
  `serve --home <absolute Windows path>` action, logon trigger, and daily local
  `03:00` trigger. `StartWhenAvailable` supplies catch-up and `ignore_new`
  prevents duplicates.
- Install/update and uninstall are idempotent; install refuses to replace a
  same-named incompatible definition. Native status detects a missing task and
  rejects definition drift. Dry-run never opens Task Scheduler.
- Native acceptance covered dry-run/status only; install/uninstall remains
  intentionally unperformed and authorization-gated.

### Slice B: Worktree model and visibility (implemented and natively accepted)

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

### Slice D: Check runner (implemented and natively accepted)

- Typed Checksets execute only applied, evidence-bound read-only commands in a
  selected Worktree, with typed argv, environment allowlist, dependency order,
  timeout/cancellation, process-tree containment, masking, and bounded evidence.
- CLI, loopback HTTP, and the embedded UI Checkset flow call the same
  application-service methods. Native Windows 11 / PowerShell 7.6 smoke passed.
- See `docs/SLICE_D_VERIFICATION.md` and commit `85bee90`.

### Slice E: Action Broker and trusted-human approval (implemented and natively accepted)

- Plans bind their complete target, inputs, executable identity, prechecks, and
  postchecks to a digest; the Broker enforces policy, locks, idempotency,
  immutable events, and immediate Worktree revalidation.
- The protected, empty-body-only UI ceremony is the sole approval path. On
  Windows, it opens `MessageBoxW` from server-derived persisted-plan metadata;
  no CLI, API, MCP agent, scheduler, or request body can grant approval.
- Non-Windows builds fail closed when native human approval is unavailable.
  A successful ceremony does not run an Action; process execution, target
  mutation, and postchecks remain separate, unimplemented work.
- WSL/cross verification and native Windows full tests/vet/build/module/race,
  interactive modal, loopback UI, Worktree fail-closed, and Scheduler
  dry-run/status checks passed. No real Action process or Scheduler
  install/uninstall was performed.

Only after these slices should actual company-specific pre-PR, release,
Jenkins, and cleanup procedures be onboarded in Milestone 4.

The generic Milestone 4 cleanup safety base is now implemented. `CleanupCandidate`
is a read-only, Worktree-bound assessment exposed through the application
service, CLI (`cleanup list`), loopback HTTP, and the embedded UI. Every
candidate is blocked unless a configured GitHub integration confirms a merged
PR and the linked Worktree passes clean/upstream/ahead/locked/prunable safety
checks; those candidates are marked `reviewable` until a separate approved
cleanup plan is executed. See `docs/MILESTONE_4_VERIFICATION.md` and the
post-MVP integration contract. Jenkins group triggers and generic release
postchecks now use the same Action Broker; company-specific inputs remain
unconfigured.

Milestone 5 now has bounded Guidance Doctor checks, masked Agent Handoff
preview and protected terminal launch, optional model metadata, and a typed
stdio MCP adapter. Milestone 6
now persists deterministic repeated-failure safeguards. Collector, Checkset,
and Action failures share one output-free normalization path; three exact
occurrences create a proposal. The protected UI owns owner assignment, shadow
feedback, activation, rollback, retirement, and effectiveness metrics. CLI
remains read-only and MCP exposes no mutation tool. SQLite atomic failure
counts and revision-CAS rule updates preserve concurrent service/CLI writers.
Activation stores the
server-derived local human approver and time. Sources that do not yet execute
(CI connector, hook, launched Handoff verification) are not claimed. Native Windows lifecycle
acceptance remains an explicit verification gap; see
`docs/MILESTONE_5_VERIFICATION.md` and `docs/MILESTONE_6_VERIFICATION.md`.

### Slice F: typed Action execution (implemented and natively accepted)

- `ActionRun` persists exact target, digest, status, bounded masked output, and
  precheck/postcheck evidence. The Broker is the only process execution owner.
- Execution consumes only server-owned typed definitions with argv, an
  allowlisted child environment, explicit Worktree directory, timeout, process
  tree containment, and output limits.
- The application service refreshes read-only Worktree evidence before and
  after execution. UI, CLI, and HTTP expose plan listing, explicit Worktree
  execution trust, execution, and result review without exposing a generic
  command surface.
- WSL tests, race tests, vet, module verification, Linux build, Windows
  amd64/arm64 cross-builds, and the native Windows runtime gate passed.
  Company release procedures, cleanup mutation, and Scheduler mutation are not
  part of this slice.

See `docs/SLICE_F_VERIFICATION.md`.

## Historical decisions left for the former implementation sequence

This is retained historical context for Milestones 0–6. Do not treat it as the
active decision queue; the active unresolved choices are in
docs/AI_CODE_ASSURANCE_PLAN.md. Resolve the following only when maintaining the
historical feature area:

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
- `internal/app/safeguard.go`: repeated-failure normalization and safeguard lifecycle.
- `internal/app/environment.go`: Agent Profiles, Environment Doctor, scheduler
  service operations.
- `internal/app/config.go`: user-local config and forward migration.
- `internal/app/web.go`: loopback HTTP adapter and UI/API routes.
- `internal/app/web_ui.go`: embedded UI asset delivery and mutation-token
  injection.
- `internal/app/ui/`: Korean application shell, responsive styles, and thin
  browser adapter over the existing loopback APIs.
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
