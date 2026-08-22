# Slice B verification: Worktree model and visibility

Implemented in schema v6 (v4 identities; v5 primary-ID trigger; v6 durable path-fingerprint repair).

## Contract

- Worktrees are scoped by `(project_id, repository_id, worktree_id)`.
- The registered checkout always has the reserved ID `primary`.
- Linked IDs derive from a SHA-256 fingerprint of verified Git common-directory
  and worktree Git-directory metadata, never from a checkout path.
- Collection uses `git worktree list --porcelain -z`; paths are never parsed by
  newline. Git-advertised paths receive only `verified_read_only` discovery
  trust after canonical-path and common-directory checks. Trust is not Action
  or Check execution authorization.
- A successful complete enumeration tombstones absent identities. A failed
  enumeration leaves existing membership intact. Prunable advertised paths are
  never entered or status-read.

## Surfaces

- CLI: `devroom project worktree list <project-id> <repository-id> --json` and `show`
- HTTP: `GET /api/projects/{projectID}/repositories/{repositoryID}/worktrees`
- Snapshot: per-repository `worktrees`; UI uses a read-only expandable details
  row while retaining existing repository summary fields.

## Verification run in WSL

```text
gofmt -w <touched Go files>
go test -count=1 -race ./...
go vet ./...
go mod verify
CGO_ENABLED=0 go build ./cmd/dev-control-room
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/dev-control-room
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/dev-control-room
git diff --check
```

Focused coverage includes NUL paths, foreign-common-directory no-read,
exit-128 failure propagation, linked registered-primary identity,
verified/unverified recovery across restart, masked path persistence,
Snapshot/HTTP/CLI list-and-show equivalence, UI dynamic-field escaping, and
v3-to-v6 migration/FK/primary-conflict scope. Native Windows runtime validation
remains pending; cross-build is not a Task Scheduler or Windows Git smoke.
