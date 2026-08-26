# Milestone E verification

Status: partial; local artifact lifecycle and fixture adapters are accepted, while real technique executors are active work
Updated: 2026-08-26

The v1 technique set now has a common report contract for static/security,
mutation, property, fuzz, and targeted E2E. Reports retain mutant status,
property/fuzz seeds, corpus or reproduction information, and bounded source
references. Artifact export uses a staging directory and per-file SHA-256
verification before the archive directory is renamed. Archive state is kept in
the manifest; local deletion requires the literal `DELETE` confirmation and
leaves the manifest marked deleted.

```text
scope: WSL; isolated generic Git fixture
commands:
  go test ./internal/assurance ./internal/app ./internal/store -run 'Test(AllV1Technique|QualityRun|AssuranceAuthoring)' -count=1  PASS
```

gaps: fixture reports do not prove mutation, property, fuzz, static/security,
or targeted-E2E tools ran. The registered-executor work is tracked in
[#5](https://github.com/knowgyu/dev-control-room/issues/5); third-party
analyzer binaries, native Windows file-lock behavior, and external archive
drives were not exercised. No artifact outside the fresh fixture home was
deleted.
