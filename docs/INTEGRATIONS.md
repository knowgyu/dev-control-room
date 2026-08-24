# Integrations and user-local runbooks

This document defines the product shape for repository groups, release/CI
operations, PowerShell runbooks, and Kubernetes read-only diagnostics. It is a
design and operator guide; real company names, hosts, job names, selectors,
tokens, and scripts must be supplied in the user's local configuration.

## What belongs in the repository

The repository may contain:

- generic domain contracts and provider-neutral Action types;
- placeholder examples using names such as `sample-web` and `sample-api`;
- tests with temporary local fixtures;
- this guide and redacted command-shape examples.

The repository must not contain:

- real project, repository, Jenkins, GitHub, Kubernetes, or Harbor names;
- company URLs, namespaces, pod names, job paths, workflow run IDs, or branch
  topology;
- token values, passwords, bearer tokens, or Credential Manager secrets;
- a checked-in PowerShell script that embeds any of the above.

Actual integration definitions are user-local data under the application's
local data directory. They are not repository `.devroom.yaml` content unless a
future reviewed, non-secret export format explicitly supports them.

## Repository groups

A group is a logical set of one or more registered repositories. The same
operation can target one repository, two repositories, or an arbitrary small
set without changing the Action contract.

Example shape, using only placeholder names:

```json
{
  "id": "sample-stack",
  "repositories": ["sample-web", "sample-api"],
  "operations": ["sync-latest", "ci-build", "deploy-stage"]
}
```

The group stores stable repository identities, not volatile runtime values.
Each operation resolves the current remote, branch, commit, build number, Pod,
or deployment target immediately before execution.

## Stable configuration versus runtime resolution

Persist stable logical intent:

- repository identity or Git remote-derived owner/name;
- GitHub workflow ID/path or a user-approved workflow selector;
- Jenkins logical pipeline or a reviewed wrapper runbook;
- Kubernetes API endpoint, namespace, workload reference, and label selector;
- a PowerShell script reference and named parameter definitions;
- environment and credential references.

Resolve at runtime:

- the current default-branch HEAD;
- the latest eligible Jenkins/GitHub run;
- the current Pod selected by labels or workload ownership;
- a deployment revision, image digest, or rollout state;
- ephemeral run IDs and Pod names.

The UI should show the resolved target before an external or mutating Action is
approved. The Action digest binds the approved logical input and resolved
preconditions. If the target changes before execution, the plan becomes stale
and must be previewed again.

## Credentials

Integration definitions store references, never resolved values. Supported
reference forms are intentionally provider-neutral:

```text
env:JENKINS_USER
env:JENKINS_PASSWORD
env:K8S_BEARER_TOKEN
windows-credential-manager:<user-local-target-name>
```

The concrete Windows Credential Manager adapter is a separate implementation
slice. Until it exists, use an explicitly named environment variable and mark
the connector as unavailable if the variable is missing. Values are resolved
only at the last responsible moment, passed only to the bounded connector or
Action process, and masked from logs, events, UI, CLI, HTTP, MCP, and handoffs.

The local configuration surface is generic and provider-neutral:

```text
GET    /api/integrations
POST   /api/integrations
PUT    /api/integrations/{id}
DELETE /api/integrations/{id}
POST   /api/integrations/{id}/check
```

Each definition stores an endpoint, kind, non-secret target values, and an
optional credential reference. Token-like value keys are rejected; put those
names in `credentialRef` instead. The UI keeps the same distinction visible.
The check endpoint resolves only `env:` references for now, performs a bounded
GET, and returns status metadata without response body or credential material.
`credential_manager:` remains an explicit unavailable result until its native
adapter is implemented.

## Initial operations

The first implemented group operation treats the repositories of the selected
Project as one logical group. It is available only after a fresh observation:

```text
POST /api/projects/{project-id}/repository-sync/plan
POST /api/projects/{project-id}/repository-sync/execute
  {"planIds":["<persisted-plan-id>"],"requestId":"<caller-request-id>"}

devroom project sync plan <project-id>
devroom project sync execute <project-id> <persisted-plan-id> ...
```

Planning creates one persisted Action plan per eligible primary Worktree and
returns a separate skip reason for every other repository. Execution accepts
only those persisted plan IDs, runs through the Action Broker, and returns one
bounded result per target. The current implementation uses two concurrent
local Git processes and runs `git pull --ff-only --prune`; it does not merge,
rebase, force-update, or touch a dirty Worktree.

### Git repository group: sync latest

For each selected repository, the operation may:

1. fetch and prune;
2. verify a clean Worktree, an upstream, and a non-detached branch;
3. pull only when a fast-forward-only update is possible;
4. report skipped repositories and reasons separately.

Repositories can run concurrently, but the group result is not successful when
one member failed or was skipped. No merge, rebase, force update, or dirty
Worktree mutation is inferred from a button labelled “latest”.

### GitHub/Jenkins CI and release

The first implementation should support both of these sources without forcing
one into the other:

- a reviewed local CLI/PowerShell runbook with exact argv and named parameters;
- a typed REST adapter using a configured endpoint, operation, and credential
  reference.

“Latest” is resolved at runtime. A release or external trigger is one grouped
Action for the selected repository set, with one approval ceremony, per-target
results, bounded output, and a postcondition such as a successful run or
expected deployment state. A specific Jenkins job need not be hardcoded if the
existing user runbook already owns that selection.

### Kubernetes logs and status

The initial Kubernetes surface is read-only:

- query workloads, Pods, readiness, rollout conditions, and recent events;
- select Pods by namespace, workload reference, or label selector;
- fetch bounded container logs through the Kubernetes REST API;
- show the resolved Pod name and observation time as result metadata only.

No `kubectl exec`, arbitrary shell, or Pod-name configuration is required.
Bearer token references, TLS settings, and API paths remain user-local. Later
mutating operations must use the same Action Broker and high-impact approval
rules as release operations.

### User PowerShell runbooks

Runbooks accept named values but execute typed argv, for example:

```text
pwsh -NoProfile -File .\scripts\release.ps1
  -Environment {environment}
  -Version {version}
  -Repositories {repositories}
```

The actual script path, allowed parameters, and values are user-local. The UI
must not accept an arbitrary shell string at click time. A runbook is reviewed,
bound to its source digest where applicable, and invoked through the Action
Broker with prechecks, timeout, masking, and postchecks.

## Redacted examples to add later

When adding a real operator example, replace every value that identifies the
company or environment:

```text
repository: example-owner/example-repository
jenkins endpoint: https://jenkins.example.invalid/
kubernetes endpoint: https://k8s.example.invalid/
namespace: example-stage
selector: app=example-api
credential: env:EXAMPLE_TOKEN
```

Keep the command shape and parameter names if they explain the integration;
remove the actual values. A real command can be supplied later as a separately
reviewed user-local fixture and must not be copied into the repository.
