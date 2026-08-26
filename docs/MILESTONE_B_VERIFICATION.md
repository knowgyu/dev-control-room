# Milestone B verification

Status: partial; automated/fake scope and real local Codex acceptance are verified, resilience remains active work
Updated: 2026-08-26

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

## Evidence

```text
scope: WSL; fake provider and typed launcher contract
commands:
  go test ./internal/assurance ./internal/store ./cmd/dev-control-room -run 'Test(Codex|FakeProvider|AssuranceLifecycle|ProjectWorktreeListJSON)' -count=1  PASS
  go test ./internal/app ./internal/assurance -run 'Test(FakeProvider|Codex)' -count=1  PASS
```

## v0.8.0 status update

`scripts/verify-real-codex.ps1` executed the current native Windows binary
against a disposable local Git fixture and passed 46 assertions. It proved the
regular `node.exe` + verified package bin path, typed argv, real authenticated
invocation, structured-only result, bounded evidence, clean Git status, and
prompt non-persistence. No company repository was contacted.

The remaining [#3](https://github.com/knowgyu/dev-control-room/issues/3) scope
is non-TTY/closed stdin, expired auth/approval prompt, explicit process-tree
cancellation/timeout, crash/reboot interruption, and idempotent resume. Those
are not inferred from this successful read-only fixture invocation.
