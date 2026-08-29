# v0.12.0 verification record

Status: PASS.
Date: 2026-08-30

## Scope

This minor release reports binary version `0.12.0`, redesigns the embedded UI
around a repository operations ledger, embeds Pretendard Variable 1.3.9, and
limits published portable artifacts to Windows amd64.

Windows arm64 remains a source-compatibility cross-build in the Full gate and is
not a release asset. Linux remains a repository CI environment and is not a
portable release target.

## Required source gates

Run from the native Windows checkout:

```powershell
pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.12.0-final
pwsh -NoProfile -File .\scripts\verify-native-resilience.ps1
pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1
pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.12.0 -OutputDirectory artifacts\0.12.0
```

## Required browser acceptance

- all six hash routes load without warning/error console entries;
- first-use State has one folder-selection action and no repeated onboarding;
- project registration opens, the native folder picker is cancel-safe, and
  closing registration restores focus;
- Work keeps observation, evidence, approval, and execution boundaries visible;
- Assurance filters change/reset without losing route state;
- Diagnostics recheck completes and optional unconfigured Providers stay
  neutral;
- Activity has an accessible caption and scoped column headers;
- keyboard focus, narrow width, long paths, and reduced-motion behavior remain
  usable;
- the font request is local, returns `font/woff2`, and no external font request
  occurs.

## Final evidence

The release candidate passed the following native checks on Windows amd64 with
Go 1.26.7, Node 24.15.0, and PowerShell 7.6.4:

- Full gate: PASS. This includes formatting, `go test -count=1 ./...`, JavaScript
  syntax, `go test -count=1 -race ./...`, `go vet ./...`, `go mod verify`,
  `go build ./...`, Windows amd64 and arm64 compatibility builds, and
  `git diff --check`.
- Native resilience: PASS, 15 assertions. Process cancellation, timeouts,
  bounded streams, restart recovery, and interrupted-invocation retry passed.
- Phase 2 journeys: PASS, 327 assertions. First use, two repositories,
  Findings, a synthetic local Provider invocation, Quality Run evidence,
  Action approval, persistence across restart, secret masking, UI contracts,
  and the local font route passed.
- Packaging: PASS. The output directory contained one portable binary archive,
  `dev-control-room_0.12.0_windows_amd64.zip`, plus `SHA256SUMS`. The archive
  contained the amd64 executable, release and verification documents, the main
  license, third-party policy, and `licenses/Pretendard-OFL-1.1.txt`. It did not
  contain arm64 or Linux executables. The adjacent `SHA256SUMS` is the
  authoritative package checksum so the ZIP does not attempt to contain its own
  recursively changing hash.

The candidate was also exercised in the Codex in-app browser against the real
loopback server:

- all six routes (`상태`, `프로젝트`, `작업`, `검증`, `진단`, `활동 기록`)
  resolved to one visible view and one `h1`, with no document-level horizontal
  overflow;
- first use displayed one folder action and one compact `준비 상태` line;
- project registration open/close state, `aria-expanded`, and focus restoration
  were checked;
- the Work decision flow remained usable at a narrow viewport (485 CSS pixels,
  the smallest width exposed by the browser viewport adapter in this run);
- the skip link moved focus to `main-content` while preserving the active route;
- Assurance restored `days`, `provider`, and `effect` from the hash URL, and its
  no-evidence state hid the otherwise empty filter and record surfaces;
- the browser loaded version-pinned local CSS and JavaScript and computed the
  body font from the embedded Pretendard Variable family.

## Explicit limits

- The browser adapter did not expose a separate historical console-log export,
  so absence of console warnings is not claimed as an independently recorded
  artifact. Route rendering and API failures were checked through visible state,
  DOM state, and the automated HTTP journey.
- The native folder-picker OS dialog was not opened during automation. Its API,
  cancel-safe JavaScript path, registration panel, and discovery journey were
  verified without automating an external Windows dialog.
- Real company Provider, GitHub, Jenkins, Kubernetes, production, and destructive
  operations were not executed. Synthetic local fixtures cover those contracts
  without broadening the release claim.
