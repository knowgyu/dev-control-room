# Milestone 5 verification: Guidance, handoff, and MCP

Updated: 2026-08-24

## Implemented

- Mechanical Guidance Doctor checks only the selected observed Worktree and
  only the fixed `AGENTS.md`, `CLAUDE.md`, and `GEMINI.md` files. It bounds
  file size, reports missing references and duplicate instructions, and never
  returns guidance contents.
- Agent Profiles retain a reviewed command, launch mode, environment allowlist,
  data boundary, and optional model argument template. CLI, HTTP, and UI share
  the same profile service.
- Agent Handoff preview returns masked summaries, exact selected scope,
  verification command names, and working-directory metadata. Transcript
  collection is always false; no launch or unmanaged process is performed.
- The stdio MCP adapter exposes only typed project, finding, cleanup,
  guidance, and handoff-preview tools. It has no generic shell, file-read,
  Action approval, or Action execution tool.

## Checks

```text
go test -count=1 ./...
```

The MCP test exercises `initialize`, `tools/list`, and a typed tool call. The
Guidance test covers bounded reference checking and handoff transcript
exclusion. Existing masking and Action-broker tests remain part of the suite.

## Boundary

The CLI, Guidance, handoff preview, and typed stdio MCP passed the later native
Windows fixture gate. Automatic Agent launch and provider-specific MCP client
acceptance remain operator checks. The current implementation deliberately
stops at preview; it does not launch Codex, Claude, Gemini, or `claude-local`
from a generic unreviewed command.
