# Dev Control Room v0.17.0 (planned)

Status: **PREPARATION — NOT RELEASED**
Prepared: 2026-09-10
Latest verified release: `v0.16.0`

This is the planned M1 project-oriented setup slice. It is a source and
documentation preparation record, not a publication or release claim. Main
must select the final source SHA, run the required verification, and perform
the separately authorized packaging and publication steps.

## Planned M1 scope

- Project-oriented setup reads the selected registered checkout and presents
  detected languages, components, recognized configuration, evidence files,
  package-manager hints, and bounded setup guidance.
- Automatic detection and manual language selection are display preferences.
  A manual filter narrows relevant recommendations without hiding other mixed
  repository components. The selection is scoped to the checkout identity and
  evidence/HEAD state; changed evidence returns the view to automatic detection.
- Python/FastAPI, Vue, JavaScript/TypeScript, and Go configuration evidence is
  recognized from an explicit manifest/configuration allowlist. Observed
  configuration, setup-needed, unsupported, and ambiguous states remain
  distinct; a manifest does not prove runtime readiness, test coverage, or
  successful execution.
- Setup previews identify the component environment, proposed command shape,
  and possible configuration/lockfile changes. They are display-only. M1 does
  not install packages, write configuration, execute Python/Vue analysis, or
  invoke an AI provider.
- Existing Go root-only actual checks remain available through the prior
  `go vet` and test/coverage paths. M1 does not turn Python/FastAPI or Vue into
  executable analyzers.

## Bounded and read-only evidence

The setup inspector reads only the validated selected checkout. Its M1 budget
is maximum depth 3, 2,000 directory entries, 64 recognized/eligible files, and
128 KiB per file. It skips the documented generated/dependency/cache
directories, does not follow links or special files, and reports partial or
warning state when evidence is incomplete. JavaScript/Python configuration is
identified and parsed only within the allowlisted formats; it is never
executed. No registry access, source upload, dependency installation, or
automatic setup occurs during detection.

## Prior fixes carried into the planned slice

- The first-use UX keeps project registration connected to a target-bound
  quality entry point and distinguishes repository refresh from running checks.
- Optional tool/configuration detail uses progressive disclosure, while empty,
  offline, retry, focus, and narrow-layout behavior remain explicit UI states.
- Bounded repository discovery recognizes a directly selected Git root,
  preserves roots found before an entry/depth/read boundary, returns partial
  warnings instead of silently discarding the result, reads directory entries
  in bounded batches, and does not follow symlink directories.

## Not included in this planned release

- Python/FastAPI or Vue analyzer/test execution and normalized findings.
- Automatic package installation, configuration writes, lockfile changes, or
  AI-assisted setup proposals.
- A new generic shell runner, arbitrary source upload, or new external
  authority.
- Release publication, remote tag mutation, or packaged-artifact evidence.

The Windows release target remains one amd64 ZIP plus `SHA256SUMS`. Packaging,
archive/hash smoke, native Windows acceptance, CI evidence, and publication are
pending main validation and explicit release steps.
