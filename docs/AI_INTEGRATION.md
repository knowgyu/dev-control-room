# AI and agent integration

## Principle

AI is a first-class client of Dev Control Room, but not a privileged client.
Humans, CLI callers, MCP clients, hooks, and coding agents all receive the same
findings and invoke the same checks and Action broker.

## Initial integration: CLI and Agent Handoff

The first AI-friendly interface is the structured CLI. It is portable across
Codex, Claude, Gemini, and local-model setups and remains useful when an agent's
plugin or MCP support changes.

When deterministic work is insufficient, the UI offers `Ask Agent`:

1. The user selects an Agent Profile and optional model.
2. Dev Control Room creates a bounded, masked handoff document.
3. The user previews its data scope.
4. The app opens the configured CLI in the explicitly selected primary or
   linked Worktree directory.
5. Dev Control Room records launch metadata and later verification results, not
   the full private conversation by default.

Handoff content may include finding identifiers, evidence excerpts, relevant
diff paths, failed commands, and required verification commands. It must not
include secret values or unrelated repository content.

## Agent profiles

Initial built-in profile templates are:

- `codex` -> `codex`;
- `claude` -> `claude`;
- `gemini` -> `gemini`;
- `claude-local` -> the user's PowerShell profile command backed by Qwen.

The templates do not hardcode current model names. A profile contains a user
configured command, optional model argument template, data-boundary label,
environment allowlist, and launch mode. The selected model is recorded as
metadata when known.

Commands defined as PowerShell aliases or functions require a deliberate
PowerShell-profile launch mode. Direct executable invocation remains the safer
default.

## Discovery assistance

Repository setup is deterministic discovery first, not agent-generated
configuration. Dev Control Room extracts existing package/build scripts,
formatter/linter configuration, CI invocations, Jenkinsfiles, and reviewed
local scripts from one explicitly selected Worktree. It emits a proposal with
source evidence, branch, HEAD, and relevant file digests and performs no install
or repository mutation.

If evidence is ambiguous, the user may ask a selected Agent Profile to draft a
proposal from the bounded discovery bundle. The draft is marked as inference,
validated against the same schema and current Worktree evidence, and cannot
apply itself. Suggestions for tooling that does not already exist are presented
separately as improvements, never disguised as discovered checks.

## MCP adapter

MCP is a later adapter, not the domain architecture. It runs over stdio and
offers narrow tools backed by the same application service. Planned tool groups:

- project and repository status;
- findings and evidence;
- environment-health summaries;
- Worktree-aware Checkset discovery and execution;
- Action planning and approval-status inspection;
- cleanup candidate inspection;
- Agent Handoff preparation.

There is no generic command execution or unrestricted file-read MCP tool. An MCP
client cannot approve its own high-risk action.

## Hooks, skills, and future quality roles

Core deterministic behavior remains in Dev Control Room commands. Provider-
specific hooks or skills should only decide when to call those commands and how
to present their structured results.

```text
Agent hook/skill -> devroom check run pre-pr --json -> normalized result
```

Specifier, Cleaner, Hardener, QA, and CRAP-style workflows are deferred. If
added, they are thin provider adapters consuming stable checks and findings,
not duplicated implementations inside prompts. This avoids coupling core
quality rules to any model or agent version.

## AI batch policy

Scheduled AI use is opt-in per Project and task type. The user selects an Agent
Profile/data boundary. AI may:

- cluster already-masked repeated failures;
- propose a deterministic safeguard;
- identify possibly stale guidance with evidence;
- summarize a complex, read-only diagnostic result.

AI may not automatically activate safeguards, rewrite instructions, close
issues, delete branches/worktrees, or perform release operations.
