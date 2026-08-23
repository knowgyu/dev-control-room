# Slice C verification: deterministic discovery proposals

Date: 2026-08-23

## Delivered contract

Slice C adds a small, read-only discovery surface over one selected, current
`verified_read_only` Worktree. It only reads:

- `package.json` string scripts, emitted as `npm run <script>` proposals; and
- `.github/workflows/*.yml` and `*.yaml` unambiguous single-line `run:` values.

Malformed package manifests and multiline YAML commands are skipped rather than
guessed. Discovery reads at most 1 MiB from each known source file, never runs
the extracted command, creates no repository file, installs nothing, and does
not scan outside the selected Worktree.

`Discovery` is the versioned response schema. `Proposal` is the durable review
record containing Project, Repository, Worktree, branch, HEAD, source-relative
path, SHA-256 source digest, command identity, deterministic inference marker,
and lifecycle state.

Proposal IDs include scope, HEAD, source digest, and command identity. This
makes revised evidence a new immutable proposal and leaves prior evidence in
the ledger. Reads report a pending proposal as `stale` when its Worktree is no
longer current/verified, its HEAD changes, or its source digest changes; the
state is durably recorded only in the protected review path. Temporary Git or
source-read failures are reported as unavailable and never rewrite proposal
lifecycle state. Only pending proposals can become `applied` or `rejected`.
Applying in Slice C records review only; it does not create a Checkset or run a
command. That execution boundary remains Slice D.

## Surfaces

- CLI: `devroom project discover <project> <repository> <worktree>` and
  `devroom proposal list|show|apply|reject`.
- HTTP: read endpoints for scoped proposal lists and individual proposals;
  loopback-token-protected POST endpoints for discovery and review transitions.
- Application service: CLI and HTTP use the same `Discover`, `Proposals`,
  `Proposal`, `ApplyProposal`, and `RejectProposal` methods.

## Verification performed in WSL

```text
go test -count=1 ./...
go test -count=1 -race ./...
go vet ./...
go mod verify
go build ./cmd/dev-control-room
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/dev-control-room
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/dev-control-room
git diff --check
```

All passed with Go 1.23.12. A temporary real Git fixture was registered and
scanned through the CLI; discovery returned exactly two proposals (`npm run
test` from `package.json`, `npm test` from the workflow), and the fixture's
`git diff --exit-code` passed afterwards. Tests cover malformed manifests, inline/multiline YAML rejection (including
list-item block scalars such as `- name: |` containing fake `- run:` text),
source/HEAD stale behavior, source symlink escape rejection, fresh Git
association checks, one-time concurrent review state/event, SQLite migration,
masking, and HTTP mutation-token enforcement.

## Native-Windows gap

The two Windows binaries cross-build successfully, but native Windows 11 /
PowerShell 7.6 runtime smoke has not been run from this WSL session. In
particular, path handling and the loopback HTTP surface need that native smoke;
cross-compilation is not a substitute.

## Deliberately deferred

Make/Task/Just/build-language parsing, Jenkinsfile parsing, documentation
imports, AI drafting, Checkset creation, and any command execution are not
implemented in this slice. They require their own evidence/parser or the Slice
D execution contract; none are inferred from repository text.
