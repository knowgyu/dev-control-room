# Dev Control Room v0.10.2

Windows-local patch release that makes the product benefit visible on the
established Home view through an evidence-first Assurance summary.

## Included

- Home now shows Quality Run, Agent execution, effect-record, and cost-status
  counts in one compact Assurance summary.
- Home now shows verified effects, trace-complete effects, and recorded or
  user-estimated time saved when the existing Assurance data supports them.
- Missing evidence remains `확인 불가`; measured and estimated time are kept
  separate. `효과 추적 보기` opens the detailed traceability view.
- The summary explains the existing proof boundary: a verified effect needs
  its source, artifact, and successful reverification at the same adopted HEAD.
- The existing local-first safety model, Codex npm launcher policy, and
  startup troubleshooting flow are unchanged.

## Windows use

1. Extract the ZIP matching the machine architecture.
2. Verify it against `SHA256SUMS`.
3. Double-click `dev-control-room.exe`, or run
   `.\dev-control-room.exe serve` in PowerShell.
4. Open <http://127.0.0.1:38471>.
5. If startup fails, follow the displayed `다음 명령` or run
   `.\dev-control-room.exe troubleshoot`.

## Evidence and boundary

The verification record identifies the exact source commit, tool versions,
commands, package hashes, and PASS/not-run results. A fresh local UI was
inspected at desktop and narrow widths, including the populated Home
effect-proof summary. Full Tab/Shift+Tab/Enter/Space traversal, native dialog
Esc delivery, assistive technology, second-device acceptance, company
endpoints, production, and destructive operations remain outside this
release's automated claim.
