# Dev Control Room v0.18.0

Updated: 2026-09-11

## Repository quality workflow

- Added a repository-scoped inspection plan for a selected Project, Repository,
  and Worktree. The plan records detected components, checks, HEAD,
  configuration evidence, and tool digest instead of assuming every repository
  is a Go repository.
- Added supported Python (`ruff`, `pytest`), Node (`eslint`, `vitest`), and Go
  inspection runners. A check is included only when its project configuration,
  verified executable, and local package state make it runnable.
- Added deterministic repository quality scores with confidence, component
  results, stale/inconclusive states, and evidence-bound score comparison.
- Added reviewable quality-improvement proposals. Findings can suggest only
  enum-based check additions/removals; applying a proposal revalidates the
  Worktree and commits the plan/proposal transition atomically.
- Added latest-result artifact restoration so refreshing Assurance does not
  erase the last run for the selected Worktree.

## Tool installation boundary

- Added server-owned previews and Action Plans for reviewed tools. The exact
  executable, package version, affected files, tool configuration digest, and
  writable paths are persisted in the plan.
- Python installation prefers the selected repository's `.venv`/`venv`.
  Global installation requires explicit opt-in and the normal human approval
  ceremony.
- Node installation uses verified `node.exe` with resolved `npm-cli.js` rather
  than invoking a shell shim. Installation remains behind the Action Broker.
- Rejected or cancelled approval responses cannot unlock execution.

## Release boundary

- Release assets contain one Windows 11 amd64 ZIP and `SHA256SUMS`.
- Linux and arm64 remain CI/cross-build concerns only; they are not published
  release assets.
- Inspection-plan generation never executes a package manager. A real install
  or external operation still requires its dedicated reviewed Action Plan and
  human approval.
