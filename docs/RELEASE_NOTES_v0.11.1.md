# Dev Control Room v0.11.1

Windows-local patch release carrying the native process-boundary work and
publishing one Windows amd64 portable package.

## Included

- The CLI and served UI report version `0.11.1`.
- Release packaging produces `dev-control-room_0.11.1_windows_amd64.zip`
  and its SHA-256 manifest. ARM64 is retained for verification cross-builds,
  but is not published as a release package.
- Provider and diagnostic commands receive an immediate EOF stdin instead of
  inheriting an interactive console.
- Windows timeout and cancellation terminate the attached child process tree
  through a Job Object.
- The explicit interrupted-invocation retry, assurance evidence, and
  fail-closed approval boundaries from the preceding native-resilience work
  remain in place.

## Windows use

1. Download the amd64 ZIP and verify it against `SHA256SUMS`.
2. Extract it and double-click `dev-control-room.exe`, or run
   `.\dev-control-room.exe serve` in PowerShell.
3. Open <http://127.0.0.1:38471>.
4. If startup fails, follow the displayed `다음 명령` or run
   `.\dev-control-room.exe troubleshoot`.

## Evidence and boundary

The verification record identifies the exact source commit, tool versions,
commands, package hash, and PASS/not-run results. Real company Provider,
Jenkins, GitHub, Kubernetes, production, destructive operations, full
keyboard traversal, and second-device acceptance remain outside this
release's claim.
