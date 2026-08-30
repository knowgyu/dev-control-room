# Dev Control Room v0.13.1

Windows amd64 patch release for the local repository control room. This release
carries the v0.13 product-surface work and corrects the last visible spacing
defect in the diagnostics page.

## Included

- Diagnostic provider rows keep the state label in a bounded 116–140px column,
  so provider names and recovery actions stay aligned on wide displays.
- Empty diagnostic context slots are no longer reserved, and the required-tool
  health message uses a compact content-sized row.
- The embedded CSS and JavaScript URLs are pinned to `v0.13.1` so a running
  browser does not retain the previous patch's layout assets.
- The first-use guide, Explorer-backed Windows folder picker, typed Jenkins MCP
  boundary, evidence-ledger UI, and shared release-version package from v0.13
  remain included.

## Preserved boundaries

Loopback-only serving, credential references, masking, Worktree identity,
Action Broker approval, idempotency, and the Windows amd64-only portable release
policy remain unchanged. ARM64 is verification-only, Linux is CI-only, and no
generic shell or unrestricted file-read MCP tool is included.
