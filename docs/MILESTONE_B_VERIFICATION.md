# Milestone B verification

Status: partial; automated/fake scope, restart-boundary recovery, and real local Codex acceptance are verified, native resilience remains active work
Updated: 2026-08-27

## Delivered

- Provider readiness distinguishes optional/unconfigured, detected, trusted
  launcher, profile readiness, and native acceptance required.
- Codex npm shims are resolved to a validated local `node.exe` plus the
  `@openai/codex` package's declared relative `codex.js`; `cmd.exe`, `.cmd`,
  and `.bat` execution are not part of the typed command.
- Fake Claude/Gemini/fixture adapters cover success, malformed output, timeout,
  cancellation, authentication, unexpected approval prompt, usage omission,
  nested launch, and provider failure.
- Agent invocations persist structured output, usage where reported, failure
  codes, bounded artifacts, and a Resume Brief. Raw transcripts remain false.
- A service restart treats queued/running/cancelling invocations as an explicit
  interruption boundary. It clears the lease, records
  `provider.interrupted`, updates the owning session's Resume Brief, and never
  relaunches a provider automatically.
- A user-directed retry is available from the Assurance UI, CLI, and protected
  API. It requires a new bounded one-line prompt, stores no prompt text, links
  the new child invocation to its interrupted parent, and uses a deterministic
  idempotency key so a repeated request does not launch another Provider.

## Evidence

```text
scope: WSL; fake provider and typed launcher contract
commands:
  go test ./internal/assurance ./internal/store ./cmd/dev-control-room -run 'Test(Codex|FakeProvider|AssuranceLifecycle|ProjectWorktreeListJSON)' -count=1  PASS
  go test ./internal/app ./internal/assurance -run 'Test(FakeProvider|Codex)' -count=1  PASS
  go test ./internal/app -run 'TestStartupRecoveryMarksActiveInvocationInterruptedWithoutRelaunch' -count=1  PASS
```

The restart test seeds a durable running invocation, closes the service, and
reopens the same local state. It verifies the interrupted state, failure code,
cleared lease, pending invocation ID, and actionable Resume Brief. Reopening
the service a second time does not create another transition or provider run.

## v0.10.3 status update

The explicit retry boundary is focused-verified with the fake Provider. A
successful retry creates a distinct succeeded child with the interrupted
invocation as `parentId`, removes the original pending item from the Session
Resume Brief, and leaves only the input digest and output evidence. The
original and new prompt strings are absent from the persisted child JSON.
Repeating the same retry returns the same child without increasing the
invocation count; a different prompt for that idempotency key is rejected.
Newline prompts are rejected before execution.

The UI renders the retry form only for `interrupted` records, shows the execution
and parent IDs, and refreshes the dashboard after submission. CLI help exposes
`assurance invocation retry --id <id> --prompt <한 줄>`. The POST route remains
protected by the local mutation token and loopback Origin check.

Focused commands:

- `go test -count=1 ./internal/app -run 'TestRetryInterruptedInvocationCreatesIdempotentChildWithoutPromptPersistence|TestEmbeddedUIAssuranceInvocationRetryRouteRequiresMutationToken|TestEmbeddedUIAssuranceDashboardContract'` — PASS
- `go test -count=1 ./cmd/dev-control-room -run 'TestNestedCLIHelpIncludesUsageAndRequiredArguments|TestAssuranceLifecycleCLIUsesNamedFlagsAndStableEnvelopes'` — PASS
- `node --check internal/app/ui/app.js` and `git diff --check` — PASS

This slice is an explicit new attempt, not old-process resume. Native process
existence/tree inspection after crash or reboot, non-TTY/closed-stdin behavior,
expired authentication or approval-prompt handling, and native timeout/
cancellation acceptance remain under
[`#3`](https://github.com/knowgyu/dev-control-room/issues/3).

## v0.8.0 status update

`scripts/verify-real-codex.ps1` executed the current native Windows binary
against a disposable local Git fixture and passed 46 assertions. It proved the
regular `node.exe` + verified package bin path, typed argv, real authenticated
invocation, structured-only result, bounded evidence, clean Git status, and
prompt non-persistence. No company repository was contacted.

The remaining [#3](https://github.com/knowgyu/dev-control-room/issues/3) scope
is non-TTY/closed stdin, expired auth/approval prompt, explicit process-tree
cancellation/timeout, native crash/reboot process inspection, and resuming an
old provider process. The new user-directed retry is idempotent as a fresh
child attempt; it does not claim to inspect or resume a provider process that
may have survived outside the service.
