# Post-MVP execution plan

Updated: 2026-08-25

This is the next implementation batch for Dev Control Room. It groups the
remaining product work into one resumable sequence, while keeping each stage
independently testable and fail-closed. Do not activate or repair any OMX
Ultragoal, Autopilot, or historical goal state for this plan.

## Goal

Complete the first usable release-project workflow:

1. finish the Korean UI flow and result review experience;
2. add grouped external work with standard Jenkins REST support and a
   PowerShell fallback;
3. execute only explicitly approved, safe cleanup candidates;
4. connect release/post-deploy evidence and refresh the handoff/release docs;
5. verify the complete batch independently with the Terra High verifier before
   native Windows operator acceptance.

The implementation must preserve the existing application-service boundary,
Action Broker boundary, masking, local-first storage, and no-real-identifiers
repository policy.

## Current evidence and constraints

- Milestones 0–3D and the generic Milestone 4–6 foundations are implemented;
  the remaining scope is recorded in `docs/RESUME_ROADMAP.md`.
- UI acceptance is explicitly pending in `docs/NATIVE_WINDOWS_SMOKE.md`.
- Cleanup is currently a read-only candidate queue; no mutation endpoint exists
  (`docs/MILESTONE_4_VERIFICATION.md`).
- Jenkins latest-build lookup and PowerShell runbook planning exist, but Jenkins
  triggers and grouped external work do not.
- Read `AGENTS.md`, `docs/HANDOFF.md`, `docs/VERIFICATION_PLAYBOOK.md`,
  `docs/INTEGRATIONS.md`, `docs/PRODUCT.md`, `docs/ARCHITECTURE.md`,
  `docs/CONFIGURATION.md`, `docs/AI_INTEGRATION.md`,
  `docs/IMPLEMENTATION_PLAN.md`, and `THIRD_PARTY_POLICY.md` before editing.
- Do not stage `.agents/` or `skills-lock.json`.

## Workstream 0: reconcile the handoff before feature work

Bring `docs/HANDOFF.md`, `docs/RESUME_ROADMAP.md`, `docs/ROADMAP.md`, and
`docs/NATIVE_WINDOWS_SMOKE.md` into agreement with the current `HEAD`, remote,
version, and accepted evidence. Preserve historical evidence, but clearly mark
superseded checkpoints. Record this plan from the current source baseline.

Acceptance:

- `git status --short --branch`, `git rev-parse HEAD`, and `git ls-remote`
  agree with the handoff's current-state section;
- every remaining item has one owner/workstream and one verification boundary;
- no document claims native Windows or release evidence that was not actually
  recorded.

## Workstream 1: Korean UI usability and result review

Use the existing embedded UI files (`internal/app/ui/index.html`,
`internal/app/ui/app.js`, and `internal/app/ui/app.css`) and existing application
service routes. Keep the surface one screen at a time rather than adding a new
frontend framework.

Implement or finish:

- clear `홈`, `프로젝트`, `작업`, `진단`, and `기록` navigation;
- Korean-first labels, empty states, errors, confirmations, and action status;
- result-first cards showing target, Worktree/HEAD, status, exit code, bounded
  masked output, prechecks, postchecks, and audit events;
- usable Project/Repository/Integration/Runbook removal confirmations;
- keyboard focus, narrow-width layout, disabled states, and no raw JSON as the
  primary result surface;
- one consistent loading/error/retry pattern for provider calls and Actions.

Acceptance:

- the fixture service opens each primary screen independently;
- a successful, failed, cancelled, and stale Action are distinguishable;
- Project and Repository removal requires the existing confirmation safeguards,
  removes registry state only, and never deletes source files;
- `node --check internal/app/ui/app.js` and the embedded UI syntax checks pass.

## Workstream 2: grouped external work

Add a generic user-local external-work group that reuses the Action Broker and
can contain two or more targets. Do not hardcode any company repository, Job,
URL, environment variable, or credential value.

### Standard Jenkins target

- accept a completed-build URL and parse its Jenkins base URL plus nested
  `/job/...` path; discard the build number, `/console`, and API suffix;
- preserve Jenkins base paths such as `/jenkins` and folder segments;
- support no-parameter `/build` and parameterized `/buildWithParameters`;
- resolve username and API token from user-configured references, with a
  default placeholder such as `env:JENKINS_USERNAME` and
  `env:JENKINS_TOKEN`, while allowing the user to change the variable names;
- use standard Basic Auth with `username:apiToken`; keep nonstandard proxy/Bearer
  behavior explicit rather than silently assuming it;
- trigger, track Queue, resolve the actual build number, and poll build status;
  do not identify the triggered build by blindly reading `lastBuild`;
- return bounded per-target status, build number, URL, result, and failure
  reason without exposing credentials or raw unbounded response bodies.

### Group behavior

- one immutable plan and one approval ceremony for the group;
- default parallel execution with bounded concurrency;
- one result per target, including partial failure and timeout;
- overall success only when every required target succeeds;
- optional named parameters per target, validated and typed before approval;
- use the existing PowerShell runbook as the fallback for plugin-specific,
  multi-step, or organization-specific Jenkins procedures;
- keep the group reusable for future GitHub/release targets without inventing a
  visual workflow engine.

Acceptance:

- a fixture accepts a build URL from a nested folder and triggers a fake Jenkins
  target through queue-to-build completion;
- two, three, and four targets produce independent results under one plan;
- one target failure does not hide the other target results;
- changed target configuration or credential reference invalidates the plan;
- secrets never appear in persisted config, UI, CLI, HTTP, MCP, events, or
  Action output.

## Workstream 3: approved cleanup execution

Extend the existing cleanup candidate model instead of creating a second
cleanup system. Add a plan/execute path through the Action Broker for eligible
linked Worktrees and branches only.

Required safeguards:

- re-observe every target immediately before planning and execution;
- reject primary, dirty, untracked, detached, locked, prunable, missing-
  upstream, ahead-of-upstream, stale, or incompletely observed targets;
- bind exact path, repository, branch, HEAD, candidate identity, and policy to
  the immutable plan digest;
- require explicit human approval and idempotency;
- remove only the exact approved linked Worktree/branch target;
- return per-target results and audit events; never imply that a dry run mutated
  anything;
- keep remote branch deletion out of the first implementation unless a separate
  explicit policy and confirmation contract is added.

Acceptance:

- safe linked fixture cleanup succeeds and is recorded;
- every unsafe fixture state remains blocked;
- a changed Worktree or candidate makes the plan stale;
- repeated execution cannot delete a second or different target;
- no production or user-important path is touched by automated tests.

## Workstream 4: release and post-deploy evidence

Reuse grouped external work, runbooks, and the existing Action result model.
Implement only generic contracts and redacted examples:

- release target/group preview with exact resolved inputs;
- stage and production as separate risk/policy paths;
- production always high-impact and explicitly approved;
- per-target Jenkins/GitHub/runbook result collection;
- postcondition evidence such as successful build, expected revision, or
  configured read-only Kubernetes status;
- clear distinction between planned, approved, running, succeeded, failed,
  timed out, and postcheck-failed states;
- update `docs/INTEGRATIONS.md`, `docs/CONFIGURATION.md`, `docs/PRODUCT.md`,
  `docs/ROADMAP.md`, `docs/HANDOFF.md`, and `docs/NATIVE_WINDOWS_SMOKE.md` with
  neutral examples only.

Do not publish a real release, deploy production, install Scheduler, or run
destructive cleanup during automated or fixture verification.

## Workstream 5: verification and independent review

For each workstream, run targeted tests first. At the batch boundary run:

```text
gofmt -w <changed Go files>
go test -count=1 ./...
CGO_ENABLED=1 go test -count=1 -race ./...
go vet ./...
go build ./...
go mod verify
node --check internal/app/ui/app.js
git diff --check
```

Use the Windows Go toolchain in an NTFS temporary checkout when WSL cannot run
the native toolchain. Keep WSL checks, Windows cross-builds, and native Windows
runtime acceptance as separate evidence.

After implementation and before declaring completion, hand the complete diff,
plan, changed docs, test output, and known gaps to an independent Terra High
verifier. The verifier must:

1. inspect the current tree and diff rather than trusting the implementation
   summary;
2. check Action Broker, approval, masking, stale-plan, path, and credential
   boundaries;
3. review grouped partial-failure and cleanup fail-closed behavior;
4. run or inspect the full verification commands;
5. return `PASS`, `PASS WITH GAPS`, or `FAIL` with file/line evidence;
6. require fixes and a second verification pass for any release-blocking
   finding.

Only after Terra High verification passes should the user perform the native
Windows ChatGPT acceptance described in `docs/NATIVE_WINDOWS_SMOKE.md`.

## Stop conditions

Stop and report instead of guessing when:

- a real credential, production endpoint, or external release permission is
  required;
- cleanup scope cannot be proven exact and safe;
- a provider's behavior requires an undocumented plugin-specific contract;
- native Windows evidence cannot be obtained;
- Terra High identifies an unresolved security, data-loss, or policy-boundary
  defect.

## Next-session command

Paste this as the first instruction in the next coding session:

```text
Read AGENTS.md, docs/HANDOFF.md, docs/VERIFICATION_PLAYBOOK.md,
docs/INTEGRATIONS.md, docs/PRODUCT.md, docs/ARCHITECTURE.md,
docs/CONFIGURATION.md, docs/AI_INTEGRATION.md, docs/IMPLEMENTATION_PLAN.md,
THIRD_PARTY_POLICY.md, and docs/POST_MVP_EXECUTION_PLAN.md.

Execute docs/POST_MVP_EXECUTION_PLAN.md end-to-end in the listed order.
Implement, test, format, commit, and push each coherent workstream while
preserving user-created .agents/ and skills-lock.json as untracked. Do not
activate Ultragoal, Autopilot, or historical .omx goal state. Do not use real
production credentials, production deployment, Scheduler install/uninstall,
release publication, or destructive cleanup during fixture verification.

At the end, run the complete verification commands from Workstream 5 and stop
implementation. Then perform an independent Terra High verifier pass over the
current diff, tests, docs, security boundaries, grouped external work, and
cleanup behavior. If Terra High reports a release-blocking finding, fix it,
rerun the affected tests, and have Terra High verify again. Finish with a
concise evidence report containing commits, push status, test results, native
Windows gaps, and the exact next action for the separate Windows acceptance.
```

