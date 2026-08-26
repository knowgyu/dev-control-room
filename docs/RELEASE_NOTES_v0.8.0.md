# Dev Control Room v0.8.0

Windows-local release for evidence-backed code assurance and a clearer daily
control-room flow.

## Highlights

- Runs Codex only through typed `node.exe` and a verified
  `@openai/codex/bin/codex.js`; it never executes `cmd.exe`, `.cmd`, `.bat`, or
  a bare `codex` launcher.
- Stores bounded structured invocation results and evidence without raw
  provider transcripts. A disposable local Git fixture passed real authenticated
  Codex acceptance.
- Adds registered native Go Quality Run runners for static/security, property,
  fuzz, and targeted E2E. Missing mutation tooling remains visibly blocked.
- Reads GitHub branch protection/rules through fixed read-only `gh.exe` argv
  and preserves unavailable provider authority as `unknown`.
- Adds first-use, return, and Provider-recovery paths; concise Korean copy;
  duplicate-warning grouping; focused finding/recovery navigation; and an
  Assurance dashboard that shows repeatability, evidence, effects, usage, and
  no-transcript boundaries.

## Install and verify

1. Download the ZIP matching your Windows architecture and extract it.
2. Verify the archive against `SHA256SUMS`.
3. Run `.\dev-control-room.exe version --json` and then `.\dev-control-room.exe`.
4. Open <http://127.0.0.1:38471>.

The release assets include portable Windows amd64 and arm64 ZIPs, SHA-256
checksums, this release note, the v0.8.0 verification record, and the recorded
first-use/return screenshots.

## Verification

The candidate passed the complete native Windows verification runner, a
239-assertion clean-state journey, and a 46-assertion real local Codex
acceptance. See [VERIFICATION_v0.8.0.md](VERIFICATION_v0.8.0.md) for commands,
environment, boundaries, and unexecuted checks.

## Explicit boundaries

This release does not accept company endpoints, production work, automatic
approval, generic shell execution, raw transcript storage, full provider
resilience, native mutation tooling, full keyboard traversal, second-device
accessibility, patch materialization, artifact restore/quota, or trend/export
scope. Those follow-ups remain public issues rather than implied capabilities.
