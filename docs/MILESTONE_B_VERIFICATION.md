# Milestone B verification

Status: accepted for automated/fake-provider scope
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

gaps: authenticated native Codex/Claude/Gemini CLI, non-TTY/closed-stdin,
Windows process-tree cancellation, and provider-specific model probing require
the native Windows checklist. No real provider, external endpoint, or write to
the user's provider configuration was performed.

