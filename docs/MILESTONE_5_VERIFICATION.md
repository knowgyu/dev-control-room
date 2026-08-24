# Milestone 5 verification: Guidance, handoff, and MCP

Updated: 2026-08-25

## Implemented

- Mechanical Guidance Doctor checks only the selected observed Worktree and
  only the fixed `AGENTS.md`, `CLAUDE.md`, and `GEMINI.md` files. It bounds
  file size, reports missing references and duplicate instructions, and never
  returns guidance contents.
- Agent Profiles retain a reviewed command, launch mode, environment allowlist,
  data boundary, and optional model argument template. CLI, HTTP, and UI share
  the same profile service.
- Agent Handoff preview returns masked summaries, exact selected scope,
  verification command names, working-directory metadata, a preview digest,
  and the exact argv contract. A protected UI/CLI launch revalidates that
  digest, starts the selected CLI in the selected Worktree, and records only
  launch metadata. Transcript collection is always false.
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

## 2026-08-25 continuation verification

The current uncommitted Handoff-launch slice was verified with Windows Go
1.26.7 from an NTFS temporary copy because the WSL checkout has no Linux Go
toolchain:

- `go test -count=1 ./internal/app`: PASS;
- `CGO_ENABLED=1 go test -count=1 -race ./internal/app`: PASS;
- remaining package tests and race tests: PASS;
- `go vet ./...`, `go mod verify`, and `go build ./...`: PASS;
- Windows amd64 and arm64 builds: PASS;
- `node --check internal/app/ui/app.js` and `git diff --check`: PASS.

These are source/build checks from a Windows toolchain. Native Windows manual
launch of a real Codex, Claude, Gemini, or `claude-local` profile is still an
operator smoke test and is not claimed here.

## Boundary

The CLI, Guidance, handoff preview, and typed stdio MCP passed the later native
Windows fixture gate. Native Agent launch and provider-specific MCP client
acceptance remain operator checks. Launch is limited to the reviewed profile,
the preview digest, the selected Worktree, and the allowlisted environment; it
does not expose a generic shell or collect a transcript.
