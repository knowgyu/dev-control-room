# Project-oriented analysis: implementation decisions

Status: design review / implementation planning; not released.

## User outcome

A Python/FastAPI + Vue user selects a checkout, confirms its language/component
configuration, receives relevant setup guidance, and runs analysis without
having to understand Campaigns, Runners, or executable inventories. Go remains
supported but is not the application's default target language.

## Boundaries

- One shared flow and result contract, small built-in analyzer adapters. No
  dynamic plugin marketplace, arbitrary shell runner, or parallel policy layer.
- Deterministic manifest/config discovery first, explicit user override second,
  optional AI proposal for ambiguity third. Multiple components/languages are
  valid; selecting one language must not conceal the rest of a mixed repository.
- Existing configuration and package manager win over recommended defaults.
  Configuration found, executable found, and successful execution are different
  facts. Missing tests is not a pass or simply a missing tool.
- Read only a bounded set of relevant manifests/configuration under the selected
  validated checkout. Do not execute configuration, traverse links, read secrets,
  contact registries, or install packages while detecting a project.
- AI receives a previewable, masked, bounded evidence bundle through configured
  agents. Its suggested plan is untrusted and must pass the same schema, scope,
  freshness and human approval checks. AI cannot approve its own installation or
  execution. No hardcoded Astra API integration or automatic source upload.
- Persisted executable plans eventually bind component, environment, HEAD,
  relevant configuration digests, executable identity, arguments and output
  parser. Changed evidence invalidates readiness; cached results remain dated.

## Incremental delivery

1. Project setup: language/component discovery and manual selection, applicable
   existing checks and concrete setup previews; retain existing Go execution
   without claiming new-language execution is implemented. Review and test this
   separately from installation or generic script execution.
2. Python/FastAPI and Vue adapters: approved local execution, normalized
   findings/test/coverage results, unavailable/error distinctions and rerun flow.
3. AI-assisted planning: bounded evidence preview, configured provider, validated
   proposed configuration and explicit review. No invented AI findings.

The user authorized incremental Windows amd64 releases on 2026-09-10. Each
release requires source verification, actual browser acceptance for its claims,
version/changelog alignment, successful release workflow, artifact hash check
and local smoke test. Do not publish merely to preserve an unfinished checkpoint.
Save unfinished work and verification gaps in handoff documentation instead.

## M1 contract review decisions

Sol high approved the milestone, identifying limits, identity/overrides and
status semantics as release prerequisites. Main resolves these as follows:

- Inspector budget: maximum depth 3, 2,000 directory entries, 64 eligible files,
  128 KiB per file. Skip `.git`, `node_modules`, `.venv`, `venv`, `vendor`, `dist`,
  `build`, `artifacts`, `testdata`, `.cache`, `coverage`, `.next`, `.nuxt` and
  links. Exclusions apply to child directories, not the explicitly selected
  root. Warnings/partial must disclose uninspected evidence.
- Only recognized manifests/configuration are eligible; the implementation's
  explicit allowlist is tested. JavaScript/Python configuration is never executed.
- Component identity is stable relative to the selected checkout and component
  root, not traversal order or display name. Keep observed components even when
  a manual language filter excludes their recommendations.
- M1 manual choices are UI preferences, not persisted/approved executable plans.
  Scope them to checkout identity and evidence digest/HEAD and reset on change.
  A database-backed guarded selection command is deferred until saved plans need
  it; adding mutable server state for a display filter is unnecessary now.
- `existing_configuration` means configuration observed, not runtime ready.
  `setup_needed`, `unsupported`, `ambiguous` remain distinct. Never infer runtime
  installation from a manifest, or infer test coverage/success from test files.
  Absence of test configuration is described as not found within inspected scope.
- Read-only `GET /api/quality/setup` takes registered project/repository/worktree
  IDs and optional supported language selections; it never accepts arbitrary
  filesystem paths or executes `commandPreview`/`setupPreview` content.
- AI controls are deferred rather than represented by fake inference/readiness.

## Visual implementation

Reuse local Pretendard and existing neutral/teal tokens. The defining element is
a compact component setup list: backend (Python/FastAPI), frontend (Vue), each
with observed evidence and one next action. Language choices use ordinary labels
and allow automatic detection or manual correction. Tool details are secondary;
installation previews identify the environment, directory, commands, affected
files and whether the app can actually apply them. No decorative hero or giant
inventory of irrelevant missing tools.

## Execution follow-up constraints (M2)

Keep the core independent of language-specific UX. A reviewed adapter resolves
one component/environment and emits explicit executable/argument arrays plus a
bounded parser contract. Its output feeds common findings, test outcomes and
coverage, not new per-language dashboards. Existing user configuration is
preserved; no replacing lint rules merely to fit a default recipe.

M1's manual language choice filters relevant recommendations; it is not proof
of a framework or a saved environment override. For empty or incorrectly
detected components, M2 needs explicit reviewed component/environment selection
and setup proposals, with observed and user-selected facts stored separately.
Do not describe the M1 display filter as completed bootstrap/configuration.

For Python, interpreter/virtual-environment selection must be explicit and
project scoped. For Node tools, prefer installed project-local dependencies;
never use an invocation that silently downloads a package to make a missing
tool appear ready. Tests and executable configuration are user code, so the
execution preview must identify that boundary even when the launcher is fixed.
Installation, lockfile changes and configuration writes are separate reviewed
actions, never side effects of detection or the first analysis request.

Unknown/partial parser output is not an empty passing report. Preserve bounded,
masked raw evidence and a parser error alongside any trustworthy partial result.
Only compare compatible target/configuration states, and show the basis of a
comparison rather than promising a causal improvement score.

## Primary-source checks for adapter design (2026-09-10)

### M2 review outcome

Sol high recommends Ruff + project-local ESLint as the first actual cross-stack
adapter pair, followed by pytest/Vitest. The current registry is technique-keyed,
execution is rooted at the checkout, and result handling is Go-specific. Amend
these explicitly rather than aliasing Python/JS failures to Go outcomes.
Use adapter identity, validated component root, expected evidence/config digest,
parser identity and typed outcomes (`clean`, `findings`, `tests_failed`,
`tool_error`, `inconclusive`). Normalize bounded findings while retaining masked
raw evidence. Do not reconstruct or execute arbitrary package-script strings.

Main security follow-up: a fixed launcher is not a sandbox. ESLint/Vitest
configuration and Python test plugins can execute user code, and configuration
can reference files beyond the initial manifest. The execution/approval design
must disclose this, validate known config dependencies, and must not claim an
environment allowlist enforces filesystem or network isolation. Resolve this
before wiring new execution buttons, not after a "read-only" release claim.

Ruff exposes machine-readable JSON and SARIF output; ESLint includes a JSON
formatter. This supports small output adapters feeding one common UI rather
than rebuilding dashboards per tool. It does not imply their output schemas or
failure semantics are identical, or that an arbitrary installed version is
supported without checking it.

- [Ruff command configuration](https://docs.astral.sh/ruff/configuration/)
- [ESLint formatters](https://eslint.org/docs/latest/use/formatters/)
- [Vue testing guidance](https://vuejs.org/guide/scaling-up/testing)

## Acceptance cases

### Resume integration review (2026-09-10)

The interrupted source delivery existed but was not a verified integration.
Luna workers resumed detector, API, UI, native QA and v0.17.0 documentation
with separate file ownership. These are review requirements, not passing-test
or release claims:

- Bound directory allocation as well as file contents; checking a counter after
  `os.ReadDir` has loaded an entire folder is not a memory bound.
- Treat lockfile conflicts, partial evidence and unsupported syntax
  conservatively. TypeScript configuration may use JSONC; strict JSON failure
  alone does not prove that a tsconfig is malformed.
- Command previews must represent actual checks: Ruff needs `check`, and
  Vitest should use a one-shot run. An unrelated package test script or a tool
  name inside an `echo` is not evidence of that tool's configuration.
- Keep component IDs opaque and stable from relative paths. Mask presentation
  fields explicitly rather than using recursive reflection with field-name
  exceptions.
- Restoring a manual language preference after reload must also restore the
  filtered result. In a shared Python/Go component, a Python-only selection
  must not expose Go execution buttons. Never treat an empty display filter as
  language bootstrapping.
- Keep setup summaries and limitations readable in Korean; observed
  configuration, an available runtime and a completed check are separate facts.
  The in-app guide must explain the same language-specific capability boundary.

Go 1.23-compatible filesystem revalidation is not an atomic sandbox. Detection
does not execute repository configuration; future execution adapters still need
the separate M2 approval and process-boundary review described above.

Empty checkout, Go only, Python only, Vue only, mixed nested backend/frontend,
ambiguous/malformed manifests, unsupported stack, no tests, conflicting lock
files, stale selection, inaccessible files, links escaping the checkout, missing
runtime, offline server, desktop/narrow layouts. No automatic install, analyzer,
package-manager or agent launch during detection; existing bounded read-only
Git checkout revalidation remains allowed. A test fixture's quality metrics never describe the
application repository's quality.
