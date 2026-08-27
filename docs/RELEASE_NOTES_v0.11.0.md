# Dev Control Room v0.11.0

Windows-local minor release that makes Provider process boundaries explicit
and records native timeout/cancellation acceptance.

## Included

- Provider and diagnostic commands receive an immediate EOF stdin instead of
  inheriting an interactive console.
- Windows timeout and cancellation terminate the attached child process tree
  through a Job Object. A local fixture proves that a spawned child does not
  survive either path.
- `scripts/verify-native-resilience.ps1` runs the native Windows acceptance
  for closed stdin, timeout, cancellation, restart-boundary recovery, and the
  idempotent retry child attempt.
- v0.10.3's explicit interrupted-invocation retry remains available through
  the Assurance UI, CLI, and protected API.

## Windows use

1. Extract the ZIP matching the machine architecture.
2. Verify it against `SHA256SUMS`.
3. Double-click `dev-control-room.exe`, or run
   `.\dev-control-room.exe serve` in PowerShell.
4. Open <http://127.0.0.1:38471>.
5. If startup fails, follow the displayed `다음 명령` or run
   `.\dev-control-room.exe troubleshoot`.

For source-checkout acceptance, run:

```powershell
pwsh -NoProfile -File .\scripts\verify-native-resilience.ps1
```

## Evidence and boundary

The verification record identifies the exact source commit, tool versions,
commands, and PASS/not-run results. The service does not discover or resume an
old Provider PID after a crash or reboot; an interrupted read-only run needs a
fresh user-directed retry. Provider-specific expired-auth or approval-prompt
acceptance, company endpoints, production, destructive operations, full
keyboard traversal, and second-device acceptance remain outside this release.
