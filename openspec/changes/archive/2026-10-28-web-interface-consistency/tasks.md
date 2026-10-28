## Implementation
- [x] Audit browser routes, overlays, copy feedback and keyboard access.
- [x] Unify icons, targets, message footers, interface terminology and state copy.
- [x] Correct copy result feedback and keyboard access to browser controls.
- [x] Verify desktop/touch flows, server assets, capability specs and rendered screenshots.
- [x] Sync capability specs and archive the completed change.

## Acceptance
Use synthetic fixture conversations only. Browser automation checks Chromium behavior; it does not establish screen-reader, Safari or physical-device accessibility conformance.

- 56 JavaScript checks and 11 Chromium fixture flows pass. Targeted follow-up checks cover Live scroll anchoring, late-capture isolation, completion/revocation cleanup and the appearance entry after a desktop-to-narrow transition.
- Go browser asset tests pass. Actual Chrome previews were inspected at desktop and 390px widths using synthetic data.
- Native screen-reader behavior, Safari and physical-device operation were not exercised in this browser refinement.
