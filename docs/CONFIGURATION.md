# Configuration and secret handling

## Configuration layers

Configuration is merged in the following order, with later layers allowed to
override only documented non-secret fields:

1. application defaults;
2. user-local project configuration under `%LOCALAPPDATA%\DevControlRoom`;
3. optional repository `.devroom.yaml` configuration;
4. explicit runtime flags for the current command.

The user-local layer is the default because company repositories may not allow
Dev Control Room files. Projects can be exported and imported without secrets.

## Project manifest shape

The local configuration schema is versioned as JSON (`version: 3`). Version 3
adds non-secret Environment/Connector metadata and a one-time Agent Profile
initialization marker. Version 2 files are migrated without discarding fields
written by Milestone 2 pre-acceptance builds. The domain objects use
`apiVersion: devroom/v1alpha1` and the local file stores only non-secret Project
configuration:

```json
{
  "version": 3,
  "scan_interval_seconds": 300,
  "projects": [
    {
      "apiVersion": "devroom/v1alpha1",
      "kind": "Project",
      "metadata": {"id": "sample-project", "name": "Sample Project"},
      "spec": {
        "repositories": [
          {
            "apiVersion": "devroom/v1alpha1",
            "kind": "Repository",
            "metadata": {"id": "service-a", "name": "Service A"},
            "spec": {
              "path": "C:\\work\\sample-project\\service-a",
              "checksets": {"pre-pr": ["format", "unit", "postgresql-lifecycle"]}
            }
          }
        ],
        "capabilities": {"jenkins": true, "release": true, "kubernetesReadOnly": false},
        "agentDefaults": {"profile": "claude"}
      }
    }
  ]
}
```

This is not a license to place commands or credentials in configuration.
Version 1 `workbenches` files are migrated to
Projects, and their old persisted mutation token is discarded. The current
anti-CSRF token is process-local and is never written to configuration.

Executable definitions use typed structures and are reviewed separately.

## Secrets

Supported secret sources are references to:

- Windows Credential Manager;
- explicitly named user or machine environment variables;
- a later approved local secret provider.

Project files may contain a reference such as `env:JENKINS_TOKEN`, but never the
value. Secret values are resolved only at the last responsible moment inside a
connector or Action process.

### Human and AI visibility

- Logs, events, SQLite rows, exports, CLI output, HTTP APIs, MCP results, and
  Agent Handoffs always contain masked values.
- A human-only settings screen may show that a secret exists, its source, last
  validation time, and a short non-secret identifier.
- If one-time reveal is ever implemented, it must be a dedicated local UI flow;
  it is never available through CLI JSON, HTTP automation, or MCP.

### Masking requirements

Mask known resolved secret values, authorization headers, token-shaped values,
credential-bearing URLs, and configured sensitive variable names before data
crosses a persistence or presentation boundary. Tests must cover exact values,
URL-encoded values, common header formats, and split stdout/stderr chunks.

## Environment doctor inventory

The doctor stores metadata only:

- variable name and declared purpose;
- expected scope and source;
- present/missing state;
- shape validation result without echoing the value;
- consuming connector or Agent Profile;
- last validation time and result.

It also reports duplicate definitions, source precedence conflicts, unresolved
commands, alias/function conflicts, and stale references.

## Milestone 2 profile and environment fields

The non-secret local configuration may declare environment metadata alongside
Projects. It contains names and scopes only; it never contains the value of a
variable or connector credential:

```json
{
  "environment": [
    {"name": "EXAMPLE_TOOL_HOME", "scope": "user", "purpose": "tool configuration", "profileId": "codex"}
  ],
  "connectors": [
    {"id": "example-connector", "name": "Example Connector", "secretReference": "env:EXAMPLE_CONNECTOR_TOKEN", "lastResult": "not_checked"}
  ]
}
```

Agent Profiles are stored as non-secret objects in SQLite and can be managed by
the `devroom agent profile` CLI family or the matching loopback API. A profile
contains a command name or executable path, a version-probe argument array,
timeout, launch mode, and environment-name allowlist. Values are never copied
from the parent environment wholesale into a probe process.

## Action policy defaults

| Risk | Default execution policy |
| --- | --- |
| Read-only | Automatic on schedule or request |
| Safe local | Ask once or allow per-project automation |
| External change | Confirm every planned execution |
| High impact | Explicit human confirmation every time |

Project overrides may make a policy stricter. They may not weaken hard-coded
production, destructive-cleanup, or credential-change safeguards.
