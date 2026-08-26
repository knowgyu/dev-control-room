# Dev Control Room v0.10.1

Windows-local patch release for clearer first-use focus and a shorter empty
Assurance dashboard.

## Included

- The initial page mount leaves focus available to the Skip link instead of
  moving it to the main region.
- Route changes still focus the reading region, while `main-content` is no
  longer an extra natural Tab stop.
- Closing the inline project registration form restores focus to its opener.
- An empty Assurance trend is rendered as one actionable empty state instead
  of a list of zero-activity rows.
- The focused UI contract and repeatable Phase 2 journey gate follow the new
  focus policy. Existing local-first safety, evidence classification, trace
  boundaries, and Codex npm launcher policy are unchanged.

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
commands, package hashes, and PASS/not-run results. A fresh packaged-source
loopback UI was inspected at desktop and narrow widths. Full Tab/Shift+Tab,
Enter/Space traversal, native dialog Esc delivery, assistive technology,
second-device acceptance, company endpoints, production, and destructive
operations remain outside this release's automated claim.
