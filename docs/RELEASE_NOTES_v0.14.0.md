# Dev Control Room v0.14.0

This document records the v0.14.0 preparation scope for the local quality
improvement workflow. The final gate and publication status are intentionally
not recorded here.

## Included in this scope

- Quality Objective is a user-owned aggregate with the explicit primary
  lifecycle `draft → baseline_pending → ready → running → review → adopted`.
  `blocked`, `stale`, and `rejected` are explicit exception paths. Decisions
  are human dispositions applied through the existing revision/CAS boundary;
  adoption requires a fresh improved revalidation on the current HEAD.
- Go coverage is a reviewed native runner using a server-owned absolute `.out`
  profile, `-mod=readonly`, a fixed test command, bounded profile parsing, and
  structured Quality Run evidence. A coverage signal is not treated as
  improved unless the runner configuration digest, current HEAD, and intact
  server-owned artifact are all verified; missing, mismatched, failed, or
  unverified evidence remains inconclusive or not improved.
- Quality-tools discovery is a fixed, read-only local view. It performs PATH
  lookup only and does not execute candidates, probe versions, install tools,
  persist a registry, infer an unselected target, or report execution
  readiness from discovery alone. Present candidates remain unverified.
- The UI adds a deterministic QualityHome improvement queue and objective
  detail path, clearer empty/loading/error/retry states, a read-only
  quality-tools diagnostic surface, live diagnostic announcements, and an
  explicitly non-persistent Assurance example view. Example data is excluded
  from storage, cost, and effect totals.

## Preserved boundaries

- Release packaging remains Windows amd64-only. No Linux or arm64 release
  asset is produced.
- Artifact policies remain fail-closed: a missing, deleted, out-of-root,
  symlinked, unreadable, or hash-mismatched artifact cannot establish improved
  coverage or permit adoption, retention changes, or restoration as verified
  evidence.
- Approval policies remain fail-closed: Quality Objectives, Quality Runs,
  agents, schedulers, MCP, and read-only discovery cannot grant approval.
  External, production, destructive, and other high-impact Actions still
  require the existing exact-plan, Worktree, Action Broker, and human-approval
  checks.
- No Linux/arm64 asset, generic shell surface, unrestricted file-read tool, or
  new final-gate PASS/release-complete claim is introduced by this preparation.
