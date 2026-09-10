# Dev Control Room v0.17.0

Updated: 2026-09-10

## What changed

- Project setup now detects bounded, read-only evidence for Python/FastAPI,
  Vue, JavaScript/TypeScript, and Go in a selected registered checkout.
- Automatic detection and manual language filters keep mixed-repository
  evidence visible; changed HEAD/configuration evidence resets the filter.
- Setup guidance distinguishes recognized configuration, setup-needed,
  unsupported, and ambiguous states, with the existing Go root checks retained.
- Registration, bounded repository discovery, refresh, reload persistence, and
  responsive first-use flows received the v0.17.0 UX updates.

## Support boundary

Python/FastAPI and Vue are read-only setup guidance in M1. This version does
not run their analyzers or tests, install packages, write configuration or
lockfiles, invoke AI, upload source, or add a generic shell runner. A fixture
coverage result is not main-repository coverage. The inspector remains bounded
and does not follow links or special files.

## Verification status

Native Full and final browser acceptance passed for source `998325b`; the exact
gates, fixtures, hashes, screenshots, and remaining native folder-picker gap
are recorded in [`VERIFICATION_v0.17.0.md`](VERIFICATION_v0.17.0.md).

The release target is one Windows amd64 ZIP plus `SHA256SUMS`. The bundled
verification record is a pre-package snapshot; the repository copy also records
the subsequent CI, package, and publication checks when completed.
