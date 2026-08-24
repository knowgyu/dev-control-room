# Verification playbook

This is the reusable verification procedure for Dev Control Room. It defines
what a check proves, what it does not prove, and the evidence needed before a
milestone or release claim. `AGENTS.md` routes agents here; it intentionally does
not copy this procedure.

## Tiers

Run the lowest tier that proves the claim, then add every higher tier required by
the changed boundary.

| Tier | Purpose | Can claim | Cannot claim |
| --- | --- | --- | --- |
| 0. Diff/static | Catch malformed or obviously unsafe edits | formatting, JavaScript syntax, whitespace, changed-file review | behavior, Windows runtime, release readiness |
| 1. WSL tests | Fast behavior and contract feedback | Go unit/integration behavior on the WSL toolchain; race results when available | native Windows paths, Job Objects, PowerShell profiles, Scheduler, real agent clients |
| 2. Windows toolchain | Reproduce the supported build/test environment | Windows Go tests, race, vet, module integrity, native/cross-build compilation | interactive UI, configured provider/client behavior, release publication |
| 3. Native Windows smoke | Exercise the actual supported runtime | Windows 11/PowerShell UI, CLI, MCP stdio, process and filesystem boundaries covered by the checklist | production safety, company integrations, or release assets unless separately tested |
| 4. Acceptance/release | Prove a named candidate or release | exact source SHA, isolated fixture, required acceptance flows, asset/hash and remote-release checks when performed | any flow omitted from the recorded checklist |

The repeatable native toolchain runner is:

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Fast
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full
```

`Fast` runs formatting, the normal Go suite, embedded UI syntax, and diff
whitespace checks. `Full` adds CGO race tests, vet, module verification, the
complete build, and Windows amd64/arm64 binaries. Results are written to a
temporary directory by default; pass `-ArtifactDirectory` to retain them at a
chosen evidence path. `-Format` is the only option that writes Go formatting
changes.

The runner is deliberately limited. It does not install or uninstall Scheduler
tasks, delete user data, run destructive cleanup, contact production, inspect
credentials, launch a real configured agent, or create tags/releases.

## Standard sequence

1. Read `docs/HANDOFF.md`, this playbook, the relevant milestone verification,
   and the current diff. Confirm the source SHA and that user-created files are
   out of scope.
2. Run Tier 0 and focused tests for the changed package or boundary.
3. Run the full Tier 1 suite at a phase or milestone boundary.
4. From a native NTFS checkout, run `scripts/verify.ps1 -Mode Full` for code
   changes that must run on Windows. Use the exact Go/PowerShell versions in
   the evidence.
5. Run the relevant checklist in `docs/NATIVE_WINDOWS_SMOKE.md` for UI, CLI,
   process, MCP, profile, Worktree, or Action behavior. Use a fresh temporary
   app-data root and a generic fixture.
6. For a release or acceptance claim, verify the exact candidate SHA, fixture
   isolation, binary hash, and remote tag/release/assets separately. A tag alone
   is not a release, and local compilation is not asset verification.
7. Stop when the claim is proven or a required tier is unavailable. Record the
   gap and do not promote a lower-tier result into a higher-tier claim.

## Fresh evidence record

Every milestone or acceptance entry should include:

```text
timestamp: ISO-8601 with timezone
source SHA: exact commit under test
environment: OS/build, PowerShell, Go, Node, gcc if relevant
scope: WSL, native Windows, fixture, release candidate, or published release
commands: exact commands and exit codes
results: PASS/FAIL per command and focused flow
artifacts: log, binary hash, fixture root, or release URL when applicable
gaps: checks not run and why
side effects: Scheduler/production/destructive operations, explicitly stated
```

Evidence is fresh only when it was produced from the recorded source SHA in the
recorded environment. Re-run or downgrade the claim when code, profile, model,
Worktree HEAD/state, relevant findings, or fixture changes. Never record secret
values, transcript contents, company paths, tokens, or default-user data in an
evidence artifact.

## Failure handling

- Fix the smallest root cause, then rerun the failed tier and its dependent
  checks; do not report an earlier passing run as current proof.
- If the toolchain is unavailable, use the next-best read-only check and mark
  the missing tier explicitly.
- If a fixture points at a real home or repository, stop that acceptance flow,
  preserve the data, correct the fixture, and rerun with a fresh temporary root.
- If a UI or provider flow cannot be automated safely, keep it as a manual
  native checklist item rather than adding a generic shell or browser harness.

## Native Windows checklist routing

`docs/NATIVE_WINDOWS_SMOKE.md` is the append-only evidence log and the detailed
manual checklist. Use it for:

- loopback UI navigation and protected mutations;
- CLI JSON and exit-code contracts;
- MCP initialize/tools and provider-specific stdio acceptance;
- Agent Handoff preview/launch, exact Worktree and argv, detached process, and
  transcript exclusion;
- Action Broker approval, timeout, cancellation, masking, idempotency, and
  Worktree fail-closed behavior;
- PowerShell profile and Windows process-tree behavior.

Keep generic fixtures local-only. Scheduler install/uninstall, production
actions, destructive cleanup, and release publication require their own explicit
authorization and evidence; they are never implied by this playbook.
