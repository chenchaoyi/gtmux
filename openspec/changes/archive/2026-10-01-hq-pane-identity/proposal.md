# HQ identity in pane browsers

## Why
All panes drops the radar's verified supervisor role and renders the raw tmux session name, so the legacy `hq` session looks like an ordinary agent. New HQ sessions also use an abbreviated default name.

## What changes
Carry the existing supervisor role into pane rows without adding a second detector. Show a neutral HQ identity badge on the session group and supervisor pane. Display legacy default HQ names as Gtmux HQ, preserve custom session names and raw targeting keys, and name newly created dedicated HQ sessions Gtmux HQ. No live session is renamed by this change.

## Surfaces
- terminal: additive role in panes JSON and an HQ marker in the pane tree; a clearer new-session default.
- menubar: HQ identity in All panes, using the same role and raw focus ids.
- phone: marked session groups and panes, legacy label expansion, older-core radar fallback.
- iPad: shared browser components, including grid headers.
- Web: same role, display names and markers in the pane browser.

## Design
HQ is an identity, not a status: a quiet text badge uses semantic foreground/surface colours. Existing status colours and rollups remain unchanged. Collapse, search, focus and guest scope retain their raw ids and names.
