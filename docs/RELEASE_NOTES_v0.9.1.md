# Dev Control Room v0.9.1

Windows-local patch release for a safer first launch and more actionable
storage failures.

## Included

- Double-click startup failures now show a short Korean reason, action, and
  next command instead of disappearing silently.
- `troubleshoot` checks the local home and shows the latest safe diagnostic
  record. `--json` remains a stable machine-readable error/result mode.
- Startup failures write only masked classification metadata to
  `troubleshooting/latest.json`; paths, SQL, credentials, and raw provider
  output are not recorded.
- Same-home server, CLI, and migration access uses a companion file lock with
  bounded waiting and typed busy errors. Context cancellation/deadline is kept
  distinct from a genuinely busy store.
- The existing v0.9.0 effect dashboard, traceability surface, artifact
  retention, and Codex npm launcher safety boundary remain unchanged.

## Windows use

1. Extract the ZIP matching the machine architecture.
2. Verify it against `SHA256SUMS`.
3. Double-click `dev-control-room.exe`, or run
   `.\dev-control-room.exe serve` in PowerShell.
4. Open <http://127.0.0.1:38471>.
5. If startup fails, follow the displayed `다음 명령` or run
   `.\dev-control-room.exe troubleshoot`.

The release contains portable Windows amd64 and arm64 ZIPs, hashes, this
release note, the verification record, and the native Windows smoke/playbook
documents.

## Boundary

This patch does not claim company GitHub/Jenkins/Kubernetes access, production
changes, destructive cleanup, real provider crash/resume acceptance, or a
second physical Windows device. Those remain explicit verification gaps in the
plan and handoff.
