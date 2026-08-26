# Dev Control Room v0.7.0

v0.7.0 packages the integrated assurance dashboard, repeatable Phase 2
CLI/loopback journeys, assurance lifecycle CLI, and secure Codex npm launcher
foundation.

## Included scope

- Assurance dashboard with Quality Run, Agent invocation, effect, artifact,
  usage, pricing, and provider/model filter surfaces.
- Repeatable Phase 2 CLI/loopback journey checks using a temporary app home and
  Git fixture; the check does not contact a real Provider or company endpoint.
- Assurance lifecycle CLI for provider/dashboard reads plus sessions,
  baselines, campaigns, runs, and invocations.
- Secure Codex npm launcher foundation: resolve local `node.exe` and the
  verified `@openai/codex\bin\codex.js` package entry as typed argv. It does not
  execute `cmd.exe`, arbitrary `.cmd`, or `.bat` files.

## Windows portable packages

- `dev-control-room_0.7.0_windows_amd64.zip`
- `dev-control-room_0.7.0_windows_arm64.zip`
- `SHA256SUMS` — SHA-256 hashes for the ZIP files.

Unpack one ZIP and run this exact safe command in PowerShell:

```powershell
.\dev-control-room.exe serve --home "$env:TEMP\dev-control-room-v0.7.0"
```

Open <http://127.0.0.1:38471> after the server starts. The temporary home
keeps this smoke run isolated from the default local data directory.

## Verification boundary

The recorded scope is automated tests plus local CLI/loopback and Windows
observations. It does not claim a real Codex task invocation or authentication.
It also makes no authoritative company CI endpoint assertion; company
Jenkins/GitHub/Kubernetes or production endpoints were not contacted or
verified. Packaging requires `VERIFICATION_v0.7.0.md` and stops before staging
if that versioned verification record is absent. Candidate code verification is
`8bcf1a7915d1e4997494ac7b6758ac51a7c794d7`; final release assets are rebuilt
from the tagged verification-record commit.
