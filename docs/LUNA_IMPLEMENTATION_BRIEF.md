# Luna implementation brief

Use this brief when handing the repository to the implementation model.

## Assignment

Implement Dev Control Room incrementally, beginning with Milestone 0 and then
Milestone 1 in `docs/IMPLEMENTATION_PLAN.md`. Use high reasoning effort. Do not
attempt the full roadmap in one change.

## Required reading order

1. `AGENTS.md`
2. `README.md`
3. `docs/PRODUCT.md`
4. `docs/ARCHITECTURE.md`
5. `docs/CONFIGURATION.md`
6. `docs/AI_INTEGRATION.md`
7. `docs/IMPLEMENTATION_PLAN.md`
8. `THIRD_PARTY_POLICY.md`

## Important constraints

- Target runtime is native Windows 11 with PowerShell 7.6 and no WSL
  dependency. Development happens in WSL, so native Windows verification must
  be explicitly reported rather than assumed.
- The existing Go code is a spike. Preserve useful behavior, but do not retain
  the Workbench model merely to avoid refactoring.
- Project can contain multiple Repositories.
- Findings, not raw logs, are the primary product output.
- CLI is the first stable AI interface; MCP comes only after core contracts.
- UI, CLI, and MCP must call the same application service and Action broker.
- Never persist or return secret values. Mask before persistence and output.
- Never add generic shell execution, generic MCP file access, telemetry, or
  automatic production/destructive behavior.
- Do not implement managed agent runs, Specifier, Cleaner, Hardener, QA, CRAP,
  Kubernetes, or Harbor in the first milestones.
- Do not add a dependency before recording its purpose, exact version, license,
  network behavior, and removal path in `THIRD_PARTY_POLICY.md`.

## Delivery expectations

For each milestone:

1. state assumptions and identify any company-environment input that is still
   unavailable;
2. update or add tests before claiming the exit criteria;
3. run Go formatting, tests, vet, and build;
4. provide exact PowerShell 7.6 smoke-test commands;
5. report any verification that could not be run natively on Windows;
6. keep changes scoped to the current milestone;
7. leave the repository in a state another agent can continue without relying
   on conversation history.

The first delivery should stop after Milestone 0 unless the user explicitly
authorizes continuing to Milestone 1.
