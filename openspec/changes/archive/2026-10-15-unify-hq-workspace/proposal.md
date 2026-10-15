# Unify the Mac HQ workspace

## Why
The Mac currently has two windows titled gtmux HQ. Board, knowledge, machine and usage shortcuts leave the new workspace for a second segmented reader. This duplicates navigation and hides where users should return.

## What changes
Use the existing workspace sidebar for all four destinations. Reuse feature views, knowledge actions, archive import/export and bounded CLI readers; remove the legacy window and segmented navigation. Menu-card shortcuts select their destination in the same retained workspace window. Resource pages have no HQ composer. Remove the duplicate matter-header HQ assessment disclosure; linked conversations show agent, current pane/location and status, with saved locators explicitly marked historical. Preserve drafts when switching destinations, and run resource polling only while its destination is visible.

## Non-goals
No new CLI/API, personal-record migration, real HQ operation, release or installation. Other surfaces keep their existing navigation.

## Surfaces
- terminal (including attach): unchanged; existing read commands are reused.
- menubar: unified HQ sidebar and window, with direct destination selection.
- phone: unchanged; this native Mac layout has no effect on the compact app.
- iPad: unchanged; its existing sidebar implementation is separate.
- Web: unchanged; no HTTP or web navigation changes.
