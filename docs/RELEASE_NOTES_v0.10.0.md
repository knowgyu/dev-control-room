# Dev Control Room v0.10.0

Windows-local minor release for restart-safe Assurance state and more honest
effect evidence.

## Included

- On startup, queued/running/cancelling Assurance invocations are marked
  `interrupted`, their lease is cleared, and a Resume Brief explains the safe
  next action. The service does not relaunch work automatically.
- Effect classifications remain distinct: measured, regression prevention,
  user-estimated, AI inference, and unavailable.
- Recorded measured time saved and user-estimated time saved are separate
  impact metrics. Estimates do not inflate measured benefit.
- Trace-ID-only source links participate in Provider/model filtering and
  traceability summaries.
- A measured or regression-prevention effect counts as verified only when its
  source, live artifact evidence, adoption commit, and successful
  reverification run/invocation all agree on the exact HEAD.
- Trace drill-down exposes source and reverification execution nodes plus
  adopted/reverified commit links. Unknown classifications are rendered as
  unavailable instead of being inferred as measured.
- Existing v0.9.1 startup diagnostics, typed JSON CLI errors, Codex npm
  launcher boundary, artifact retention, and progressive-disclosure UI remain
  in the package.

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

## Evidence and boundary

The release verification record identifies the exact source commit, tools,
commands, package hashes, and PASS/not-run results. The package does not claim
company GitHub/Jenkins/Kubernetes access, production changes, destructive
cleanup, automatic patch adoption, causal productivity attribution, native
crash/process-tree resilience, or full manual accessibility acceptance.
