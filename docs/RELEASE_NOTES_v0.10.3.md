# Dev Control Room v0.10.3

Windows-local patch release that makes interrupted Assurance executions
recoverable through an explicit, traceable new attempt.

## Included

- The Assurance dashboard shows a retry form only for `interrupted`
  invocations. It shows the execution ID and parent execution ID.
- The operator supplies a new bounded one-line prompt. The original prompt is
  never persisted, and newline or oversized retry prompts are rejected before
  execution.
- The UI, CLI, and protected API use the same retry contract:
  `assurance invocation retry --id <id> --prompt <한 줄>` and
  `POST /api/assurance/invocations/{id}/retry`.
- A retry is a distinct child invocation with `parentId` and a deterministic
  idempotency key. Repeating the same request returns the existing child
  without launching the Provider again.
- The startup boundary remains fail-closed. This release does not inspect or
  blindly resume an old Provider process.

## Windows use

1. Extract the ZIP matching the machine architecture.
2. Verify it against `SHA256SUMS`.
3. Double-click `dev-control-room.exe`, or run
   `.\dev-control-room.exe serve` in PowerShell.
4. Open <http://127.0.0.1:38471>.
5. If an invocation is marked `중단`, open its details, enter a new prompt,
   and select `재시도`.
6. If startup fails, follow the displayed `다음 명령` or run
   `.\dev-control-room.exe troubleshoot`.

## Evidence and boundary

The verification record identifies the exact source commit, tool versions,
commands, package hashes, and PASS/not-run results. Native process-tree
inspection after crash/reboot, non-TTY/closed-stdin, expired authentication or
approval-prompt acceptance, full keyboard traversal, second-device acceptance,
company endpoints, production, and destructive operations remain outside this
release's claim.
