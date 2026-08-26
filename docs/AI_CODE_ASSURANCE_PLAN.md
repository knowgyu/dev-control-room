# Phase 1 — AI-assisted Code Assurance plan

Status: foundational automation accepted; real-provider, authoritative-baseline, and registered-executor completion is active
Updated: 2026-08-26
Owner: local operator

This is the canonical active feature plan. It is Phase 1 of the current product
sequence. Phase 2 usability work is planned separately in
[PHASE_2_PRODUCT_USABILITY_PLAN.md](PHASE_2_PRODUCT_USABILITY_PLAN.md); it must
not displace Phase 1 acceptance work or turn an unfinished safety contract into
a UI-only release. It supersedes the deferred
Hardener/QA/CRAP wording in older planning documents, but does not invalidate
accepted milestone evidence or weaken the Action Broker, masking, local-first,
and Worktree-trust boundaries.

## Product outcome

Dev Control Room will help one developer deliberately improve assurance for
selected repositories. It discovers the real PR baseline, runs targeted quality
experiments, preserves reproducible evidence, asks the human about ambiguous
behaviour, and uses installed coding-agent CLIs to propose—never silently
adopt—test and configuration changes.

The product answers evidence-backed questions:

- Which checks really protect this repository's PRs today?
- What mutation, property, fuzz, security, contract, or E2E experiment ran at
  which commit, and what did it find?
- Which defect, survivor, counterexample, or security finding was fixed or
  prevented because of this work?
- What can continue while the operator is away, and where did it stop?

It is not a generic autonomous coding swarm or a single vanity quality score.

## Current state and plan transitions

| Existing material | Status | Treatment |
| --- | --- | --- | --- |
| Milestones 0–6 and verification files | accepted historical evidence, with recorded native gaps | Preserve as the foundation for the application service, Action Broker, masking, profiles, and UI. |
| docs/POST_MVP_EXECUTION_PLAN.md | historical implementation plan | Generic v0.5.0 source work is implemented; explicitly recorded native acceptance work remains pending. |
| docs/IMPLEMENTATION_PLAN.md | historical foundation plan | Milestones 0–6 are not an active queue. |
| docs/AI_INTEGRATION.md | active foundation, partially superseded | Existing Handoff/MCP contracts remain; managed assurance is defined here. |
| Checksets | active, retained | They remain the fast reviewed PR/baseline runner, not a mutation/fuzz runner. |
| Agent Handoff | active, retained | It remains the interactive human-led path; Managed Hardening Sessions add the non-interactive path. |

docs/ROADMAP.md is the short status index. Important policy and architecture
choices live in docs/decisions/, so old plans are not silently rewritten.

## Decisions fixed by the product discussion

1. CLI only in v1. Invoke locally installed, already authenticated Codex,
   Claude, or Gemini CLIs. Do not add direct provider API keys, API projects,
   hosted orchestration, or a new remote service.
2. Non-interactive execution is the automation path. Codex uses
   non-interactive execution; Claude/Gemini use equivalent headless modes.
   Interactive terminal Handoff stays available for human-led work.
3. Codex first. Codex CLI is the first real adapter. Claude and Gemini get
   automatic detection, profile registration, and fake-adapter coverage first.
   A detected npm `codex.cmd` launcher is supported without invoking `cmd.exe`:
   the adapter verifies a local `node.exe`, the adjacent `@openai/codex`
   package metadata and declared `bin/codex.js`, then invokes Node and that
   script as a typed argv command. It never executes an arbitrary `.cmd`/`.bat`
   file or interpolated shell command.
4. Existing authority remains. The application service, Action Broker,
   Worktree model, masking, and approval system remain the only authority for
   mutation and external Actions. AI output is evidence/proposal, not approval.
5. CLI nesting is permitted, not required. A permitted provider may launch
   another approved local CLI. Dev Control Room records the parent invocation
   and final evidence; opaque nested work is never independently verified.
6. Human review adopts patches. Deterministic safety checks can block an
   unsafe run. A critic may advise, but cannot silently accept/reject a patch.
7. No raw transcript by default. Persist structured Q&A, plan, final result,
   selected events, patch, usage, and artifacts. Raw streams are opt-in,
   retention-bound diagnostic artifacts.
8. Tool setup needs authority. Public documentation research and setup
   proposals are allowed. Package installation, lockfile/CI changes, and other
   material writes need normal approval or a prepared unattended scope.
9. No silent local-model downgrade. Local models can be deliberately chosen
   for classification/summarization, never silently substituted for the
   selected semantic test-design model.
10. Usage is visible; cost is honest. Report provider usage when present.
    Money is a versioned public API-list-price equivalent, not an asserted
    Enterprise-seat or CLI bill.

## Core workflow

### Establish the PR CI baseline

For a selected Worktree, discover and reconcile local workflow files,
package/build scripts, Jenkinsfiles, reviewed runbooks, actual GitHub PR checks
and branch rules through installed gh where configured, Jenkins evidence, and
human-declared requirements. The resulting PR CI Baseline separates required,
observed, local equivalent, and unknown entries.

A Checkset may be proposed only for the fast locally reproducible subset. It
must never invent a CI command or call an unobserved check required.

The baseline stores every source, captured time, source digest/ETag when
available, target branch, and HEAD. Provider-enforced GitHub rulesets or branch
protection are authoritative for required status; an explicit human requirement
or reviewed configured Jenkins contract can add a requirement but cannot erase
provider enforcement. Current PR/check history is observed evidence; workflow
files and package scripts are discovery evidence only. A local equivalent
requires a recorded source mapping or human confirmation. A baseline becomes
stale when its target branch/HEAD changes, an authoritative source changes,
configured source freshness expires, or a source cannot be refreshed within its
policy; unavailable information is represented as unknown, not silently absent.

### Run targeted Quality Runs and Campaigns

A Quality Run is distinct from a Checkset: it is a bounded, reproducible,
on-demand experiment against an exact Worktree, branch, HEAD, and configuration
digest. A Quality Campaign groups related runs, questions, patches, and effect
evidence.

| Technique | Primary executor | Required evidence |
| --- | --- | --- |
| Mutation testing | installed mutation tool | killed/surviving mutants, report, target tests |
| Property-based testing | installed language-native tool | property, seed, counterexample, shrink result |
| Fuzzing | installed fuzzer/runtime | corpus/seed, crash or timeout repro, minimized input |
| Static, dependency, security analysis | installed analyzer/connector | rule/advisory ID, file/line, severity, report |
| Contract/integration tests | repository tooling | producer/consumer contract and environment evidence |
| Differential/metamorphic tests | repository tooling | compared implementation or invariant transformation |
| Targeted E2E | fixture/app runner | scenario plus logs/screenshots/reports when configured |

No technique runs merely because it exists. The operator selects its purpose,
or an AI proposal explains why it is useful for the selected change. Mutation
testing is explicitly on-demand, not a required PR gate. CRAP is not an initial
metric or release gate.

The v1 runnable executor set is static/security analysis, targeted mutation,
property-based testing, fuzzing, and targeted E2E. The common Quality Run
contract intentionally leaves room for contract/integration and
differential/metamorphic adapters, but those are tracked as a follow-up rather
than silently promised in v1:
[GitHub issue #2](https://github.com/knowgyu/dev-control-room/issues/2).

### Use AI for ambiguity and test design

Deterministic evidence comes first: change summary, CI baseline, relevant
tests, tool availability, and prior Assurance Specs. The selected CLI agent may
identify risks, ask concise questions, draft an Assurance Spec, propose
properties/generators/mutation targets/fuzz inputs/contracts/tool setup, draft
a patch in an isolated Worktree, and interpret a bounded run report.

The UI stores questions and answers as structured state. A provider session may
be resumed when useful, but the local Assurance Spec is durable memory. A
Resume Brief shows current HEAD, decisions, completed/pending runs, failed
evidence, questions waiting for an answer, proposed patch, and next safe action
when the user returns after a meeting or overnight run.

### Execute and review

Deterministic tools run through typed, allowlisted Quality Runner definitions.
AI-generated tests/configuration are changed only in a temporary registered
Worktree. The comparison shows patch, baseline result, changed result, and
artifact references. The human adopts, rejects, or requests revision; managed
sessions never commit, push, open a PR, or change CI without separate authority.

An optional critic receives the Assurance Spec, patch summary, and deterministic
evidence. It returns advisory concerns/confidence, not a final verdict.

### Durable lifecycle and recovery contract

Milestone A introduces additive, revisioned records with explicit ownership and
stale digests: Assurance Session, Assurance Spec revision, Agent Invocation,
Quality Campaign/Run, PR CI Baseline, Artifact, Effect, Provider Pricing
Snapshot, and Unattended Approval Scope. A Session owns spec revisions and
invocations; a Campaign owns Quality Runs; runs and invocations reference
artifacts; an Effect links a finding/mutant/counterexample to adoption and
reverification evidence.

Sessions and runs have a fail-closed state machine: draft, awaiting answer,
ready, queued, running, cancelling, interrupted, succeeded, failed, timed out,
cancelled, stale, or expired. A persisted lease and idempotency key identify
each invocation. On service restart, logout, reboot, or expired lease, an active
run becomes interrupted after checking its owned process state; it is never
silently retried. Read-only work can be explicitly resumed or retried as a new
invocation. Any write requires fresh Worktree observation, exact plan/scope
validation, and the normal Broker boundary. Artifact writes use a staging
directory, hash/validation, then an atomic local manifest transition so partial
output is never presented as completed evidence.

## Provider, model, usage, and cost design

### Provider adapters

Every adapter implements capability discovery, non-interactive run, optional
resume, cancellation, and normalized result parsing. Provider Doctor reports
not installed, found, launchable, authenticated, structured output supported,
and usage supported; it never assumes a PATH entry is usable.

The first real adapter invokes Codex CLI in the selected Worktree with explicit
sandbox/approval settings and a structured output schema. Its event stream is
reduced in memory to selected persisted data. Codex session IDs can support
two-stage Q&A/critic work; independent work uses ephemeral mode where useful.
Claude/Gemini use the same normalized contract after their CLI capability is
confirmed. Fake adapters cover success, malformed output, timeout,
cancellation, missing usage, nested launch, and provider failure.

### Model selection

The UI presents a provider/model selector from the active Agent Profile.
Profiles carry a model-argument template, approved choices, and an optional
custom value. Provider Doctor verifies capability rather than relying on a
permanently hardcoded model list. Every invocation records requested
provider/model, profile/command version, provider-reported resolved model when
available, and whether selection was user, profile default, or unknown. The
UI/dashboard filters runs, effects, and usage by provider and requested/reported
model.

### Usage and pricing ledger

Agent Invocation Usage records provider-reported input, cached-input, output,
reasoning, tool, and total tokens where available, plus elapsed time/status.
Missing fields remain unknown, never prompt-length estimates.

Nested CLI use is recorded as parent/child relationship metadata. Directly
reported child usage is stored separately; unobservable nested usage is marked
unattributed/unknown and is never added to the parent total. Missing price
snapshots or unresolved model aliases do not block a run; only the
cost-equivalent field remains unknown.

Provider Pricing Snapshot is immutable: provider, model, official source URL,
retrieved/effective time, currency, token-unit prices, price dimensions, and
parser/manual-review status. A refresh previews the official source and asks
the operator to save the snapshot. A historical run always uses its original
snapshot; it is never recalculated with today's price. If model, price, or usage
is unknown, show usage without money.

The monetary figure is labelled estimated public API list-price equivalent; it
must never be presented as the actual cost of an Enterprise CLI entitlement.

## Evidence, effects, and storage

Each Quality Run/agent invocation records Worktree, branch, HEAD, config/tool
digests, command identity, timestamps, status, bounded masked logs, and an
artifact manifest. Artifacts carry MIME type, size, SHA-256, retention state,
and source reference. They can include reports, mutant survivors, fuzz corpora
and crashes, counterexamples, patches, screenshots, external CI/Jenkins
references, and provider summaries.

Artifacts use a local quota and time policy. Users can pin evidence, archive or
export selected evidence packs to another local path such as D:, verify hashes,
and delete with a warning about lost reproducibility/reports/logs. Summary and
manifest records survive ordinary artifact cleanup.

Archive/export is two-phase: copy to a staging destination, verify every
manifest hash, atomically mark the archive verified, then offer—not perform by
default—local artifact deletion. A missing external drive leaves the local
manifest and last verified archive state intact; it never makes the artifact
appear safely archived.

Add an embedded Korean effect surface—not a new web service—with lifetime/period
cards, daily/weekly/monthly trend, filters, evidence drill-down, CSV/JSON
export, and per-provider/model usage. Every effect is linked evidence with a
source label:

- measured: reproduced failure/mutant/counterexample/advisory linked to a
  verified fix;
- prevented regression: a reproduced defect now has an adopted protection;
- user-estimated: an operator saved a manual-time estimate;
- AI inference: a suggestion awaiting confirmation;
- unavailable: no defensible measurement.

Initial values include verified defects before merge, security/reliability
findings, survivors converted to protections, property/fuzz counterexamples,
adopted test improvements, run duration, optional calibrated time saved, usage,
and list-price equivalent. Formatting/lint results stay visible but separate
from security/reliability outcomes.

Measured effect counting is deduplicated by a stable effect fingerprint: source
finding/advisory identity, or exact run plus mutant/counterexample/crash
identity, coupled with the adopted patch/commit. A measured fix requires the
source evidence, human adoption, and successful reverification at the adopted
commit. Later runs update that effect rather than adding another headline win.

## Authority and unattended execution

Unattended Approval Scope is a reviewed envelope, not blanket access. It binds
Projects/Worktrees, allowed read/write paths, provider profiles, tool-setup
actions, techniques, network policy, disk quota, deadline, and prohibited
actions. It may permit public-doc research and approved local setup while
forbidding commit, push, PR creation, CI edits, remote dispatch, deletion, and
scope expansion unless separately approved.

When human knowledge is required, the session completes independent safe work,
stores its question/brief, then enters waiting for answer; it must not leave an
unattended terminal blocked. Existing Action Broker protections remain mandatory
for mutations and external operations.

Provider-boundary filtering excludes known secrets, credential files, .env,
private keys, database dumps, and operator-excluded paths. It records the
allowed file manifest and selection reason. This reduces accidental exposure; it
does not claim arbitrary source code lacks sensitive information.

An Unattended Approval Scope is an upper-bound policy, never a generic mutation
permission. Before every write, the service creates an exact ActionPlan and
proves it matches the human-approved scope template: exact Project/Worktree,
tool and version/config digest, argument schema and resolved values, writable
paths, risk class, deadline, disk limit, and forbidden-operation set. The
ActionPlan digest, scope digest, match result, fresh observation, and postcheck
are persisted. A mismatch, expired scope, changed Worktree/HEAD, unavailable
precondition, or unrecognised generated action fails closed and becomes a
question or blocked result.

No unattended run is eligible until native acceptance proves non-TTY/closed
stdin operation, expired authentication, unexpected provider approval prompt,
missing/invalid structured output, missing model/usage fields, cancellation,
timeout, process-tree termination, crash/reboot interruption, idempotent
resume, and partial artifact/archive recovery. It must not change Windows power
settings. Service start at logon and resume after service startup are separate
settings. Resume after startup is user-configurable and applies only to
eligible sessions: the prior process must be confirmed gone; the scope must
still be valid; the current Worktree/configuration must revalidate; and the
checkpoint must permit a new idempotent attempt. Otherwise the run remains
interrupted with a Resume Brief. The service never blindly continues an old PID
or reruns an uncertain external/high-impact Action.

## Delivery milestones

| Milestone | Scope | Exit evidence | Status |
| --- | --- | --- |
| A — plan/persistence | status markers, ADRs, migrations for sessions/runs/artifacts/pricing | migration/lifecycle tests and no duplicate active plan | accepted automated foundation; see MILESTONE_A_VERIFICATION.md |
| B — providers/continuity | Doctor, Codex adapter, structured schema, profiles/model picker, fake Claude/Gemini, Resume Brief | fake matrix; masking; cancellation; non-TTY, closed-stdin, auth/prompt/output/usage failure cases; no raw transcript default; separate native real-CLI acceptance | partial: trusted npm launcher and JSONL reduction are implemented; explicit bounded prompt, actual Codex invocation, and native acceptance remain [#3](https://github.com/knowgyu/dev-control-room/issues/3) |
| C — CI/runner | gh/Jenkins baseline discovery and typed Quality Runs | fixture proves required/observed/local equivalent/unknown and stale detection | partial: local discovery exists; provider-authoritative evidence is [#4](https://github.com/knowgyu/dev-control-room/issues/4), and registered executors are [#5](https://github.com/knowgyu/dev-control-room/issues/5) |
| D — assurance authoring | Q&A, Assurance Spec, test/property/mutation/fuzz proposals, isolated patch, critic | tests prove AI cannot adopt/commit/push and stale specs/runs surface | accepted automated boundary; see MILESTONE_D_VERIFICATION.md |
| E — techniques/artifacts | v1 static/security, mutation, property, fuzz, and targeted-E2E adapters; reports; archive/export/cleanup | three-repo fixture creates/verifies/exports/restores artifacts and deletion warning | partial: artifact lifecycle and fixture reports are accepted; real registered executors remain [#5](https://github.com/knowgyu/dev-control-room/issues/5) |
| F — effects/usage | dashboard, filters, usage ledger, pricing snapshots, exports | fixture produces correctly labelled measured/estimated/unknown effects and stable historical pricing | accepted automated foundation; the embedded evidence dashboard is included in the v0.7.0 candidate |
| G — dogfood/release | campaign on Dev Control Room and existing three-repo fixture | full automated suite, fake-provider E2E, targeted real CLI acceptance, reviewed effect report | partial: fake-provider dogfood is accepted; real CLI and clean-state journey acceptance remain [#3](https://github.com/knowgyu/dev-control-room/issues/3) and [#7](https://github.com/knowgyu/dev-control-room/issues/7) |

Implement one milestone per branch. Mark it completed here and in the Roadmap;
use superseded or stale for changed plans, never silent replacement.

## Verification strategy

Unit/integration coverage includes migrations, typed command construction,
capability detection, model/profile validation, structured output parsing,
usage/pricing arithmetic, masking, authority boundaries, artifact hashing,
archive restore, retention warnings, stale digests, and effect classification.

The existing three-repository fixture is extended with frontend/backend/database,
fake GitHub/Jenkins/Kubernetes evidence, fake provider CLIs, and seeded mutant,
property, fuzz, security, CI, patch-adoption, archive, and cleanup-warning
scenarios. This proves the product flow without cloud providers or company code.

Real CLI checks are separate native Windows acceptance: use the authenticated
Enterprise CLI on an intentionally safe repository, start read-only, and record
only structured results plus operator-approved diagnostic artifacts. Fake or WSL
success never proves real provider CLI operation.

## First implementation action

Begin Milestone A with status/decision convention and additive domain migrations
without changing Checkset or Action semantics. Build the fake-provider contract
before the first real Codex CLI invocation so that integration remains narrow,
observable, and reversible.

## Next-session assignment prompt

Use the following as a new ChatGPT/Codex task for the active Phase 1 work:

> Work only on Phase 1 in `C:\\Users\\knowgyu\\workspace_window\\dev-control-room`.
> First read `AGENTS.md`, `README.md`, `docs/HANDOFF.md`,
> `docs/AI_CODE_ASSURANCE_PLAN.md`, the relevant ADRs, and the current
> `git status`. Treat the Phase 1 plan as the scope contract. Begin at
> Milestone A unless its exit evidence is already present; inspect and report
> the existing implementation before changing anything. Preserve Checksets,
> Action Broker approvals, Worktree trust, masking, local-first storage, and
> the rule that AI patches require human adoption. Implement exactly one
> milestone or coherent vertical slice at a time. For an explicit unattended
> “continue Phase 1 to completion” request, do not stop after a successful
> milestone: commit each verified coherent milestone, update its evidence and
> status, then begin the next planned milestone. Add migrations and tests before
> real-provider calls; use fake providers for automated E2E. Run focused tests
> and the relevant Windows/native acceptance checks, inspect the diff, and
> update the plan/Roadmap status only with evidence. When a human answer is
> required, persist the question and Resume Brief, continue unrelated safe work,
> and stop only when no approved independent work remains. Do not start Phase 2
> UI work, do not push, and do not change Windows power settings. Ask before a
> security-boundary or external-state change that is not already approved by
> the plan.
