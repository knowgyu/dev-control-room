# Milestone C verification

Status: partial; local discovery fixture is accepted, while provider-authoritative baseline and registered runners are active work
Updated: 2026-08-26

The baseline reader inspects package scripts, GitHub Actions unambiguous
single-line `run:` entries,
and an explicit local required-check declaration. It labels provider-enforced
rules unknown when authoritative provider evidence is unavailable. A baseline
is stale when its freshness window expires or the exact Worktree HEAD changes.
Quality Run execution stays separate from discovered commands and uses only
registered allowlisted runners; the v0.8.0 native Go runner evidence is
recorded in Milestone E.

```text
scope: WSL; isolated generic Git fixture
commands:
  go test ./internal/app -run 'TestBaselineDiscovers|TestFakeProvider' -count=1  PASS
  go test ./internal/app -run 'TestQualityRun|TestBaselineDiscovers' -count=1  PASS
```

## v0.8.0 status update

The workflow parser now skips YAML block scalars such as `run: |` and `run: >-`
rather than saving a bogus command. The current binary performed the two fixed,
read-only `gh.exe api` lookups on this repository. GitHub returned HTTP 403 for
the private-repository branch-protection/rules feature, and the baseline kept
`github.authoritative` as `unknown` rather than inventing required checks. Raw
provider data and credential values were not persisted.

Jenkins evidence and a successful provider-authoritative required-context
example remain [#4](https://github.com/knowgyu/dev-control-room/issues/4).
Registered Quality runner scope is recorded separately in Milestone E.
