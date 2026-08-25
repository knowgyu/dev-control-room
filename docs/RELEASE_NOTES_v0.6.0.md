# Dev Control Room v0.6.0

v0.6.0 completes the local AI Code Assurance lifecycle and the Phase 2 first-use experience.

## Included

- Phase 1 Milestones A–G: durable sessions, provider continuity, baseline discovery, authoring/critic boundaries, five quality techniques, artifacts, effects, usage, pricing snapshots, and three-repository dogfood coverage.
- Phase 2: first-use and established home states, grouped optional Provider status, concise Korean copy guidance, user journeys, CLI help, assurance CLI queries, and the validated Codex npm launcher boundary.
- Windows Codex npm launcher support uses `node.exe` and the verified `@openai/codex` `bin/codex.js` with typed argv. It never invokes `cmd.exe`, arbitrary `.cmd`, or `.bat` files.

## Windows files

- `dev-control-room_0.6.0_windows_amd64.zip`
- `dev-control-room_0.6.0_windows_arm64.zip`
- `SHA256SUMS` — the authoritative ZIP hash list.

The ZIP files are portable. Unpack one and run `dev-control-room.exe serve --home <local-data>`, then open `http://127.0.0.1:38471`.

## Verification

See `VERIFICATION_v0.6.0.md` and the included `NATIVE_WINDOWS_SMOKE.md`. The release does not claim a real Codex task invocation, company credentials, production endpoints, or a second physical Windows 11 device.
