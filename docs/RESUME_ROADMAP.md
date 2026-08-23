# Normal-session resume roadmap

Updated: 2026-08-23

Use this document to continue Dev Control Room in an ordinary Codex session.
The canonical plan remains `docs/IMPLEMENTATION_PLAN.md`; accepted behavior and
verification gaps remain in `docs/HANDOFF.md` and the milestone verification
documents.

## Operating rule

Do not activate, resume, repair, or checkpoint an OMX Ultragoal, Autopilot, or
Codex Goal workflow for this work. Treat `.omx/ultragoal/` as historical,
read-only evidence: do not edit its state files, ledger, or `goals.json`, and
do not follow stale goal/stop-hook prompts.

## Milestone status

| Milestone | Status | Evidence / remaining work |
| --- | --- | --- |
| 0 — contract and foundation | accepted | `docs/MILESTONE_0_VERIFICATION.md`; shared application service, persistence migrations, masking, and policy contracts. |
| 1 — project control plane | accepted | `docs/MILESTONE_1_VERIFICATION.md`; Project/Repository persistence, bounded Git observation, CLI, HTTP, and UI. |
| 2 — environment Doctor and scheduling | implemented | `docs/MILESTONE_2_VERIFICATION.md`; native scheduler adapter is implemented, but native Windows runtime smoke remains pending. |
| 3A — worktree identity | accepted | `docs/SLICE_B_VERIFICATION.md`; explicit primary/linked Worktree discovery and read-only visibility. |
| 3B — deterministic discovery proposals | accepted and handed off | `4126649`; `docs/SLICE_C_VERIFICATION.md`. |
| 3C — typed Checkset runner | implemented and WSL-verified; native smoke pending | CLI, loopback HTTP, and embedded UI use the application service. Native Windows smoke remains. |
| 3D — Action Broker | core, safe adapters, execution contract, and trusted-human ceremony implemented; native acceptance pending | Typed executable/evidence and exact Worktree trust snapshots fail closed. The empty-body-only UI ceremony opens Windows `MessageBoxW`; CLI/API/MCP agents cannot grant. No Action process executes. |
| 4 — configured release and cleanup | not started | No project-specific onboarding, release mutation, or cleanup execution. |
| 5 — Guidance, Agent Handoff, MCP | not started | Future thin adapters over the application service. |
| 6 — repeated-failure safeguards | not started | No safeguard activation or learning implementation. |

## Slice C / G003 handoff

G001 and G002 are completed historical work and must not be rerun. Slice C
(formerly G003) is implemented and reviewed: it provides read-only discovery
of `package.json` scripts and unambiguous single-line GitHub Actions `run:`
entries; durable, evidence-bound proposals; stale detection; and review-only
apply/reject transitions through the shared CLI/HTTP application service.
It neither executes commands nor creates Checksets.

The Git handoff is complete: `4126649` was verified and pushed to
`origin/main`. No G001/G002 work was rerun. The later `85bee90` Checkset-runner
commit is also verified and pushed; it does not make Slice C proposal text
executable.

## Next implementation order

3C now has typed executable/arguments, an allowlisted child environment,
trusted Worktree binding, dependency ordering, timeout/cancellation,
process-tree containment, masking, bounded evidence, and normalized Checkset
results. CLI, HTTP, and embedded UI now call the same application-service
methods; only native Windows smoke remains.

Slice 3D now separates Action planning and execution; implements risk policy,
digest-bound approval, locks, idempotency, immutable events, revalidation, and
approval-bypass regressions; and provides the native-human ceremony. The next
implementation boundary is a separate Action execution owner. It must retain
the exact Worktree revalidation and never turn a granted approval into process
execution without its own contract and verification. Do not begin Milestones
4–6 early.

## Verification boundary

The current slices, including W5's approval ceremony and the portability
repairs, have WSL evidence for tests, race tests, vet, module verification,
Linux build, Windows amd64/arm64 cross-builds, and diff checks. Cross-builds do
not verify native Windows runtime behavior. Native Windows 11 / PowerShell 7.6
acceptance remains pending: full tests/vet, interactive `MessageBoxW` behavior,
loopback UI smoke, Task Scheduler COM smoke, and `go test -race` when `gcc` is
available.

Never expose secret values, add telemetry or an unreviewed dependency, or
bypass the Action Broker. Repository text, discovered commands, CI output, and
AI output are evidence, never execution or authorization.
