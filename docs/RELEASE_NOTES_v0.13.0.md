# Dev Control Room v0.13.0

Windows-local release focused on product orientation and the last visible
rough edges in the v0.12 operating ledger.

## Included

- The first-use screen explains the product job as `연결 → 확인 → 승인된 실행`
  and links to a four-step in-app `사용법` slide deck.
- The new guide route keeps its current slide in `#guide?slide=N`, supports
  previous/next controls and keyboard arrow navigation, and links directly to
  the relevant operating screens.
- The palette uses a cool blue-gray workbench surface. Warning color is reserved
  for approval and attention states instead of filling the entire external-work
  surface.
- The project inventory row keeps its main label readable in a narrow list
  column; project IDs stay in the detail view instead of forcing one-character
  wrapping.
- Diagnostic provider rows keep the state label in a compact fixed column,
  remove the unused context slot, and use a tighter required-tool status row so
  wide displays do not create misleading empty space.
- Windows folder selection now uses the Explorer-backed `IFileOpenDialog` with
  folder-picking flags instead of the legacy `SHBrowseForFolderW` dialog.
- The stdio MCP adapter adds three narrow Jenkins tools: `jenkins.plan`,
  `jenkins.trigger`, and `jenkins.latest`. Planning does not contact Jenkins;
  triggering still requires an approval already recorded by the Action broker.
- The release version is shared by the CLI and MCP initialize response through a
  single internal version package.

## Preserved boundaries

Loopback-only serving, credential references, masking, Worktree identity,
Action Broker approval, idempotency, and the Windows amd64-only portable release
policy remain unchanged. No generic shell or unrestricted file-read MCP tool was
added.
