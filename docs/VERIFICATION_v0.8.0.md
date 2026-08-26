# v0.8.0 verification record

Status: release candidate and local package verified; remote asset download is
the publication follow-up.

Candidate source: `a4e664304d3097aa27b79f39eabf8f96f606224e`  
Date: 2026-08-26  
Environment: native Windows, PowerShell `7.6.4`, Go `go1.26.7 windows/amd64`,
Node `v24.15.0`, Git `2.53.0.windows.1`, gcc `16.2.0`.

## Automated verification

| Command | Result | Evidence |
| --- | --- | --- |
| `pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full -ArtifactDirectory artifacts\verification-v0.8.0-final` | PASS | gofmt, full Go test, UI syntax, race, vet, module verification, build, Windows amd64/arm64 build, and diff check. Summary: `artifacts/verification-v0.8.0-final/verification-summary.json`. |
| `pwsh -NoProfile -File .\scripts\verify-phase2-journeys.ps1` | PASS, 239 assertions | Fresh temporary app homes and local Git fixtures; first-use, return, Provider recovery, local npm launcher, fake provider, registered Go runner, blocked paths, restart, duplicate warning, and secret-canary checks. |
| `pwsh -NoProfile -File .\scripts\verify-real-codex.ps1` | PASS, 46 assertions | Disposable local Git fixture; real authenticated Codex invocation through typed `node.exe` + verified `@openai/codex/bin/codex.js`; no shell launcher, raw transcript, prompt persistence, or fixture mutation. |
| Focused UI/assurance tests and `node --check internal/app/ui/app.js` | PASS | Multiline GitHub workflow parsing, dialog focus contracts, Provider recovery focus, finding deep link, project selection, and protected UI flow. |

The Full verifier built the candidate Windows binaries with these SHA-256 values:

| Artifact | SHA-256 |
| --- | --- |
| `dev-control-room-windows-amd64.exe` | `26C7E186BA0C3506896C1F3F6500F35FAABEAEC232D74EA43B4A95CB57508C28` |
| `dev-control-room-windows-arm64.exe` | `1E91ED10BCA915E5DF8EBCC1175B6E666476D8B250288A7FC59EB80291F15B5C` |

## Browser and keyboard evidence

- Fresh home shows onboarding and the primary `프로젝트 등록` action without
  advanced empty panels.
- A local Git fixture registration returns to a selected project; the selected
  card has `aria-pressed=true` and accepts `Enter`.
- A finding link moves focus to the exact finding card. Provider recovery moves
  focus to `provider-statuses`.
- Desktop, return, and 390 px screenshots were captured at:
  `artifacts/verification-v0.8.0-final/ui-first-use-final.png`,
  `artifacts/verification-v0.8.0-final/ui-return-final.png`, and
  `artifacts/verification-v0.8.0-final/ui-return-narrow-final.png`.
- Browser console error log was empty. Dialog description and explicit-cancel
  focus restoration are covered; an opener removed by re-render falls back to
  `main-content`.

## Read-only GitHub baseline evidence

The current binary read the local workflow source and attempted its fixed,
read-only `gh.exe api` branch-protection and branch-rules queries against this
repository. GitHub returned HTTP 403 because that private-repository feature is
not available for the current plan. The baseline persisted a bounded
`github.authoritative` `unknown` entry instead of inventing a required check.
No credential value or raw provider response was persisted.

## Package verification

`pwsh -NoProfile -File .\scripts\package.ps1 -Version 0.8.0
-OutputDirectory artifacts\0.8.0` passed and created both portable Windows ZIPs
plus `SHA256SUMS`. The amd64 archive hash matched the manifest; a fresh extract
reported `version --json` = `0.8.0`, returned a successful `--help --json`
envelope, and served a fresh loopback-only `/api/health` response with telemetry
disabled. The final release manifest is the authoritative archive hash record.

After GitHub publication, download the uploaded assets into a new directory,
verify them against the uploaded `SHA256SUMS`, and repeat the extracted amd64
`version --json` and loopback health smoke. That remote check is reported with
the release publication result; it does not change the source/tag evidence.

## Not run / not accepted

- Company GitHub/Jenkins/Kubernetes endpoints, production, proxy, credential
  mutation, or destructive cleanup.
- Codex non-TTY/closed stdin, expired auth, approval prompt, explicit
  process-tree cancellation, crash/reboot, and idempotent-resume acceptance.
- Successful provider-authoritative required CI contexts or Jenkins baseline.
- Native mutation tool execution; `go-mutesting.exe` was unavailable and stayed
  blocked.
- Full Tab/Space traversal, reliable native-dialog Esc automation, second clean
  Windows device, or assistive-technology review.
- Isolated patch materialization/adoption, artifact restore/quota/pin, and
  trends/export traceability.

The deferred scope is tracked in [#3](https://github.com/knowgyu/dev-control-room/issues/3),
[#4](https://github.com/knowgyu/dev-control-room/issues/4),
[#5](https://github.com/knowgyu/dev-control-room/issues/5),
[#7](https://github.com/knowgyu/dev-control-room/issues/7), and
[#9](https://github.com/knowgyu/dev-control-room/issues/9)–[#12](https://github.com/knowgyu/dev-control-room/issues/12).
