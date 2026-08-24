# Agent client examples

These examples are thin callers of the stable CLI. They do not implement
orchestration or infer commands from repository text.

```powershell
devroom project list --json
devroom finding list --project <project-id> --repository <repository-id> --json
devroom guidance check <project-id> <repository-id> <worktree-id> --json
devroom agent handoff preview --profile codex --project <project-id> --repository <repository-id> --worktree <worktree-id> --json
devroom cleanup list --project <project-id> --json
devroom safeguard list --json
```

The MCP fallback is:

```powershell
devroom mcp serve --home $env:LOCALAPPDATA\DevControlRoom
```

MCP is stdio and typed. It does not expose a generic shell or file reader, and
it cannot approve or execute a protected Action.
