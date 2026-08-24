# Dev Control Room agent instructions

This file is intentionally short. The canonical requirements and decisions are
the linked documents; do not duplicate them here.

Read, in order:

1. `docs/HANDOFF.md`
2. `docs/VERIFICATION_PLAYBOOK.md`
3. `docs/INTEGRATIONS.md`
4. `docs/PRODUCT.md`
5. `docs/ARCHITECTURE.md`
6. `docs/CONFIGURATION.md`
7. `docs/AI_INTEGRATION.md`
8. `docs/IMPLEMENTATION_PLAN.md`
9. `THIRD_PARTY_POLICY.md`

Implement one milestone at a time. Target native Windows 11 and PowerShell 7.6,
although the source workspace is currently accessed through WSL. Keep UI, CLI,
and future MCP adapters thin over one application service. Never expose secret
values, bypass the Action broker, add telemetry, or introduce an unreviewed
dependency. Run formatting, tests, vet, and build before handoff, and distinguish
native-Windows verification from checks run only in WSL.

## Verification routing

Use `docs/VERIFICATION_PLAYBOOK.md` to choose the smallest verification tier
that proves the current claim. On native Windows, prefer the repeatable runner:
`pwsh -NoProfile -File .\scripts\verify.ps1 -Mode Full`. Use `-Mode Fast` for
the local edit loop and `-Format` only when an intentional formatting write is
wanted. The runner does not install Scheduler tasks, perform destructive
cleanup, contact production, or create a release.

Native Windows UI, CLI, MCP-provider, and configured-agent acceptance remains a
separate manual tier; WSL tests and cross-builds never substitute for it. Record
the exact commit, environment, commands, results, and any gap in the relevant
verification document. Keep the detailed procedure in the playbook rather than
duplicating it here.
