# Add a collapsible browser Live strip

## Why
Parsed transcript updates can lag behind the active turn. Browser readers need to see that the agent is still working and inspect its current output, as they can in the app.

## What Changes
- Add a collapsed-by-default Live disclosure above the Chat composer, shared by focused conversations and workspace panels.
- While the agent is working, show its state and a duration derived from the server clock when available.
- Expand to read the latest 14 captured screen rows; preserve disclosure state and reader scroll intent.
- Refresh focused agent status, remove Live after work stops, and discard late responses after mode/pane changes or credential revocation.

## Surfaces
- terminal: reuse the existing read-only screen endpoint; keep the terminal input and capture contracts.
- menubar: no native changes; its local browser entry includes the Live strip.
- phone: browser touch layout receives Live; no native client changes.
- iPad: browser layout receives Live; no native client changes.
- Web: focused Chat and workspace Chat panels share Live, including read-only viewers with capture access.
