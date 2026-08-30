# Agent client examples

These examples are thin callers of the stable CLI. They do not implement
orchestration or infer commands from repository text.

```powershell
devroom project list --json
devroom finding list --project <project-id> --repository <repository-id> --json
devroom guidance check <project-id> <repository-id> <worktree-id> --json
devroom agent handoff preview --profile codex --project <project-id> --repository <repository-id> --worktree <worktree-id> --json
devroom agent handoff launch --profile codex --project <project-id> --repository <repository-id> --worktree <worktree-id> --preview-digest <digest-from-preview> --json
devroom cleanup list --project <project-id> --json
devroom safeguard list --json
```

For a Jenkins trigger, use the typed MCP tools so the plan and approval remain
visible in Dev Control Room:

```text
jenkins.plan   -> groupId + projectId + repositoryId + worktreeId
                 (reviewable plan; no Jenkins request)
jenkins.trigger -> planId + holder + idempotencyKey
                  (runs only after a human approval exists)
jenkins.latest -> integrationId
                  (read-only latest build metadata)
```

The MCP fallback is:

```powershell
devroom mcp serve --home $env:LOCALAPPDATA\DevControlRoom
```

MCP is stdio and typed. It does not expose a generic shell or file reader. The
Jenkins trigger still goes through the Action broker; an MCP client cannot
approve its own protected Action.
