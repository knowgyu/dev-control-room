# Dev Control Room v0.9.0

Windows-local release for proving the value of AI-assisted checks with
measured effects, traceable evidence, and recoverable result artifacts.

## Highlights

- Adds an Assurance effect dashboard with period, project, Provider, model, and
  effect classification scope; equal-length previous-period comparison; daily
  or weekly trend; and explicit measured, user-estimated, AI-inference, and
  unavailable states.
- Connects an effect to its source run or invocation, Finding, artifact, SHA-256,
  Trace ID, adoption commit, and reverification run without exposing local
  storage paths in trace or report output.
- Adds JSON/CSV impact reports, trace drill-down, artifact storage quota, archive
  manifests, atomic export, SHA-256 restore verification, and active/pinned
  retention controls.
- Adds `assurance artifact list|export|retention|restore|delete` for portable
  evidence-pack lifecycle management.
- Keeps the v0.8.0 safety boundary: Codex npm launcher execution uses typed
  `node.exe` plus the verified package `bin/codex.js`; `cmd.exe`, arbitrary
  `.cmd`/`.bat`, bare `codex`, raw transcripts, company endpoints, and automatic
  patch commit/push remain outside the product boundary.

## Install and verify

1. Download the ZIP matching the Windows architecture and extract it.
2. Verify the archive against `SHA256SUMS`.
3. Run `.\dev-control-room.exe version --json`.
4. Start `.\dev-control-room.exe` and open
   <http://127.0.0.1:38471>.
5. Open `검증` to review the effect scope, evidence completeness, trace, and
   artifact retention state.

The release assets include portable Windows amd64 and arm64 ZIPs,
`SHA256SUMS`, this release note, the v0.9.0 verification record, and the
browser evidence captures used during release validation.

## Verification

The source candidate passed the full Go/race/vet/build/module verification,
the 239-assertion clean-state Phase 2 journey, the focused effect/trace/artifact
tests, and loopback browser review on a Windows host. See
[VERIFICATION_v0.9.0.md](VERIFICATION_v0.9.0.md) for exact commands, evidence,
and the checks that remain intentionally unaccepted.

## Explicit boundaries

The dashboard records evidence-backed change; it does not infer causality or
personal productivity. A measured effect still needs human adoption and
reverification metadata. Native mutation tooling, patch materialization,
provider-authoritative company CI, full keyboard/assistive-technology review,
and provider crash/resume resilience remain separate follow-up work.
