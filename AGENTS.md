# Dev Control Room agent instructions

This file is intentionally short. The canonical requirements and decisions are
the linked documents; do not duplicate them here.

Read, in order:

1. `docs/PRODUCT.md`
2. `docs/ARCHITECTURE.md`
3. `docs/CONFIGURATION.md`
4. `docs/AI_INTEGRATION.md`
5. `docs/IMPLEMENTATION_PLAN.md`
6. `THIRD_PARTY_POLICY.md`

Implement one milestone at a time. Target native Windows 11 and PowerShell 7.6,
although the source workspace is currently accessed through WSL. Keep UI, CLI,
and future MCP adapters thin over one application service. Never expose secret
values, bypass the Action broker, add telemetry, or introduce an unreviewed
dependency. Run formatting, tests, vet, and build before handoff, and distinguish
native-Windows verification from checks run only in WSL.
