# Architecture

## Runtime shape

Dev Control Room is a Windows-native local background service with three thin
surfaces over one application core.

```text
 Browser UI             CLI              MCP stdio adapter
     |                   |                       |
     +-------------------+-----------------------+
                         |
                Application service
          query / command / policy / masking
                         |
       +-----------------+------------------+
       |                 |                  |
   Collectors        Reconcilers       Action broker
       |                 |                  |
  Observations ------> Findings       Plans / approvals
       |                 |                  |
       +-----------------+------------------+
                         |
       SQLite event store (modernc.org/sqlite, CGo-free)
```

UI, CLI, and MCP must not implement business logic independently. Their output
may differ in presentation, but the same command must receive the same policy,
masking, execution, and audit treatment on every surface.

## Component boundaries

### Project registry

Loads user-local Project configuration and optional repository-level
`.devroom.yaml` files. It resolves repositories and enabled capabilities but
does not hold secret values. Repository is the logical source-control identity;
Worktree is a concrete checkout path beneath it. The primary checkout and every
linked worktree have separate identities, state, and execution scopes.

Linked worktrees are enumerated from NUL-delimited Git porcelain metadata, not
by assuming a directory name. A `.worktrees` folder therefore works without
special handling. A
discovered path is canonicalized and verified against the same Git common
directory before bounded read-only observation. Discovery does not grant
mutation authority for that path.

### Collectors

Collectors gather typed observations. Initial collectors cover local Git,
worktrees, tool resolution, environment metadata, and PowerShell profiles.
Later collectors cover GitHub, Jenkins, Kubernetes read-only diagnostics, and
optional Harbor metadata. Worktree collection retains path, branch, HEAD,
dirty/untracked state, upstream drift, and detached/locked/prunable state for
each checkout rather than reducing the repository to a count. Collectors are
read-only and may not remediate.

### Reconcilers

Reconcilers derive current Findings from observations and configured policy.
They are deterministic whenever possible. An AI batch may create a proposed
finding or safeguard, but it must retain evidence and be marked as an inference.

### Discovery and proposals

Deterministic discovery inspects a user-selected worktree for existing build,
formatter, linter, test, CI, and reviewed script entry points. It produces a
proposal; it does not create configuration, install dependencies, rewrite a
command, or execute the discovered entry point. Existing package/build scripts
and CI invocations are preferred over newly reconstructed commands.

A proposal records Project, Repository, Worktree, branch, HEAD, source file and
digest, extracted command identity, and whether any field was AI-inferred. The
user reviews the proposal before it becomes a Checkset or Action definition.
Changing the selected worktree, HEAD, or relevant source digest makes the
proposal stale. Optional AI assistance can interpret ambiguous evidence, but it
cannot widen the scanned scope, activate a proposal, or mutate the repository.

### Check runner

The Check runner executes reviewed Checksets with explicit executables,
argument arrays, working directories, environment allowlists, timeouts, and
output limits. It produces normalized results and masked evidence. Each run is
bound to an explicit Worktree identity and revalidates its canonical path and
expected source state before execution.

### Action broker

The Action broker is the only component allowed to mutate local or remote
state. It performs:

1. schema and capability validation;
2. plan creation without execution;
3. risk classification and policy evaluation;
4. required approval acquisition;
5. locked execution with timeout and cancellation;
6. postcondition verification;
7. immutable evidence recording.

Approvals bind to a digest of the complete ActionPlan. Execution recomputes the
digest immediately before running so a change to a commit, branch, target,
input, or postcondition invalidates the grant. Actor authority is explicit:
agents and the scheduler may request work, but only the human UI can grant or
reject it. The single human user may confirm an action they initiated.

No UI route, CLI command, MCP tool, plugin, hook, or agent adapter may bypass
the broker.

### Scheduler

Windows Task Scheduler starts the service at logon and has one fixed local
daily `03:00` trigger. The task uses `StartWhenAvailable` so a missed daily run
catches up after the next start, and `TASK_INSTANCES_IGNORE_NEW` so the daily
and logon triggers cannot create a second service instance. The service then
performs bounded periodic jobs and records missed-run state.

### Persistence

SQLite is the target store for projects, observations, current findings,
events, action plans, approvals, runs, and failure fingerprints. Large command
output is bounded and stored only after masking. Database migrations are
forward-only and tested from the previous released schema. Repository identity
is `(project_id, repository_id)`, and all scoped rows use SQLite foreign keys
with foreign-key enforcement enabled. The migration runner rejects future
schema versions, missing migration history, renamed migrations, and sequence
gaps.

## AI-friendly interfaces

### CLI first

The stable machine interface is the CLI. Every query supports `--json`; JSON
schemas and exit codes are versioned. Initial command families are:

```text
devroom project list|show|doctor
devroom project worktree|discover
devroom proposal list|show|apply|reject
devroom finding list|show|acknowledge
devroom env doctor
devroom check list|run
devroom action list|plan|execute
devroom cleanup list|plan
devroom event list
devroom agent profile|handoff
devroom serve
```

Commands never emit secrets. Human-readable text goes to stdout, structured
errors go to stderr, and non-zero exit codes distinguish invalid input, failed
checks, denied policy, execution failure, and unavailable capability.

The stable machine envelope is `devroom/cli/v1`:

```json
{"schema":"devroom/cli/v1","ok":true,"data":{},"meta":{}}
```

Errors use a stable `error.code` and `error.message`. Exit codes are `0` for
success, `1` for internal errors, `2` for invalid input, `3` for failed checks,
`4` for denied policy, `5` for execution failure, `6` for unavailable
capabilities, `7` for not found, `8` for conflict, and `9` for forbidden.
HTTP adapters use the same envelope and application service.
Unknown errors are classified as `internal_error` with the fixed message
`internal error`; raw filesystem, SQL, command, and credential-bearing details
are never serialized by CLI or HTTP adapters.

### Local HTTP API

The embedded UI uses a loopback HTTP API. State-changing requests require a
per-install anti-CSRF token and same-origin validation. The API is not the
public agent contract in the initial release.

### MCP after the application contract

A stdio MCP adapter is added only after the CLI and application service are
stable. It provides narrow typed tools such as listing projects/findings,
running a Checkset, planning an Action, and preparing an Agent Handoff. It must
not provide a generic shell tool.

Read-only tools follow project scope. Mutating tools call the Action broker and
cannot manufacture human approval. External and high-impact actions return an
approval requirement that must be satisfied through the human surface.

Repository-scoped tools also require an explicit Worktree identity whenever
file content, diffs, checks, Actions, or an Agent working directory may differ
between checkouts. They must not silently fall back from a selected linked
worktree to the primary checkout.

The CLI remains the universal fallback for providers whose MCP support is
missing or unreliable.

## Windows and WSL boundary

The application is developed from the current WSL workspace but ships and is
accepted as a Windows binary. Runtime paths, process creation, PowerShell
profiles, credential references, and Task Scheduler integration must be tested
from PowerShell 7.6 on Windows 11. WSL path conversion is not a production
feature unless explicitly added later.

## Security invariants

- Bind network listeners to loopback by default.
- Do not add telemetry or automatic update checks.
- Treat repository files, connector text, logs, and AI output as untrusted data.
- Never interpret observed text as authorization or an executable command.
- Store secret references, never secret values, in project configuration.
- Apply masking before persistence, rendering, CLI JSON, MCP output, or prompts.
- Use an environment allowlist for actions and agent launches; do not inherit
  the parent process environment wholesale.
- Keep executable and arguments separate; reject unresolved executables and
  unvalidated working directories.
- Treat Git-advertised linked worktree paths as read-only discoveries until
  their canonical path and common-directory association are verified. Never
  use one worktree's evidence to authorize execution or cleanup in another.
- Record who or what requested an Action, the policy decision, approval, exact
  executable identity, result, and postcheck evidence.
- Recompute and verify the approved ActionPlan digest immediately before
  execution.
- Require explicit human approval for destructive and production operations.
