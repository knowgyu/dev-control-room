# Dev Control Room v0.12.0

Windows-local minor release replacing the generic dashboard presentation with a
smaller repository operations ledger while preserving the existing safety and
execution boundaries.

## Included

- The CLI and served UI report version `0.12.0`.
- The six-route shell keeps compact top navigation and labels Home as `상태`;
  no persistent sidebar is added.
- Repeated hero, eyebrow, card, metric, and decorative chip patterns are reduced
  in favor of comparable ledger rows and the product-specific
  `관찰 → 근거 → 승인` evidence flow.
- First use presents one folder-selection path instead of repeating registration
  through onboarding cards and another call to action.
- Projects, Work, Assurance, Diagnostics, and Activity use flatter inventories,
  decision queues, evidence records, capability checks, and chronological rows.
- Pretendard Variable 1.3.9 is embedded and served from the loopback origin.
  There is no runtime font CDN or telemetry request.
- The research, design contract, font license, and future AI-assisted UI guardrail
  are stored in the repository.
- Release packaging still produces only
  `dev-control-room_0.12.0_windows_amd64.zip` and `SHA256SUMS`. Windows arm64 is
  retained as a verification cross-build; Linux remains a CI environment.

## Windows use

1. Download the amd64 ZIP and verify it against `SHA256SUMS`.
2. Extract it and double-click `dev-control-room.exe`, or run
   `.\dev-control-room.exe serve` in PowerShell.
3. Open <http://127.0.0.1:38471>.
4. If startup fails, follow the displayed `다음 명령` or run
   `.\dev-control-room.exe troubleshoot`.

## Preserved boundaries

The release does not change the loopback-only server, credential-reference
rules, secret masking, Worktree trust, Action Broker approval, production risk,
or fail-closed process behavior. Real company Provider, Jenkins, GitHub,
Kubernetes, production, and destructive operations remain outside the local
release claim unless explicitly exercised in the verification record.
