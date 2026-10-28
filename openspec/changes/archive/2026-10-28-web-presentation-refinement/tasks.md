## Implementation
- [x] Add trace inspector close/reopen with state retention.
- [x] Add system/light/dark appearance and adapt all browser surfaces.
- [x] Refine browser branding, layout and tab identity.
- [x] Unify native pairing delivery and renewal/reachability presentation.
- [x] Verify desktop/narrow flows, native build and capability documentation.

## Acceptance
- 55 JavaScript unit checks and 9 Chromium fixture flows pass, covering live system appearance, explicit choice persistence, authentic terminal colors, close/reopen and polling retention for both trace surfaces, narrow layout and contextual tab identity.
- 31 Swift pairing/renewal/reachability/guest-delivery checks pass; the final native release build passes.
- Go server/transcript race tests and all 38 OpenSpec capability validations pass. Source design checks pass in a clean copy of the working tree (avoiding ignored sibling-worktree false positives).
- The actual local preview was inspected in Chrome using synthetic records. Closing details expands the list, and switching Dark/System updates the same page. The preview service was restarted at its existing loopback-only address to update its favicon and wrapper theme.
- The installed App was not replaced. The refactored native pairing windows were compiled and contract-tested; live native clicking, QR scanning and phone pairing were not exercised.
