## Implementation
- [x] Inspect native Chat Live data and lifecycle behavior.
- [x] Add a shared collapsible Live strip, focused status refresh and current-screen preview.
- [x] Preserve disclosure/scroll/draft state and guard responses by view, pane, turn and credential.
- [x] Verify lifecycle, late responses, desktop/touch layout and rendered output.
- [x] Sync capability specs and archive the completed change.

## Acceptance
Automated and visual checks use synthetic conversations and captures. No input is sent to installed agents.

- 56 JavaScript checks and 11 Chromium fixture flows pass. Targeted follow-up checks cover Live scroll anchoring, late-capture isolation, completion/revocation cleanup and the appearance entry after a desktop-to-narrow transition.
- Go browser asset tests pass. Actual Chrome previews were inspected at desktop and 390px widths using synthetic data.
- Native screen-reader behavior, Safari and physical-device operation were not exercised in this browser refinement.
