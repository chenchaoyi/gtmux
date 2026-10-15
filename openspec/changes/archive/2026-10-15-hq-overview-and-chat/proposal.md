# HQ overview and readable conversation

## Why
The Mac workspace currently offers both matter state groups and an authored situation
board as competing views of current work. The board mixes historical handoffs with
pending decisions, so its highlighted attention section can look current after it is
stale. First reads also show empty content or generic spinners, matter state icons
are mostly gray, and conversation controls differ from the mobile app.

## What changes
- Make a structured Overview the default destination and the menu-card board shortcut.
  Use the same matter groups for Overview and sidebar; keep the complete authored
  board as a secondary, dated, read-only Handoff snapshot without live-state emphasis.
- Use existing status glyphs/colors and independently collapsible sidebar categories.
- Reuse the gtmux mark for initial loading and compact refresh feedback; distinguish
  loading, failed reads, valid empty results and retained content.
- Share chat bubbles across HQ and source readers: avatar/name outside the bubble,
  Copy full text below, and time separators at the first known time, day changes or
  gaps of at least 15 minutes, matching mobile.

## Surfaces
- terminal: no CLI, lifecycle, personal-record or delivery behavior changes.
- menubar: native HQ navigation, reading and feedback changes described above.
- phone: no code changes; existing conversation patterns are the reference.
- iPad: no code changes; existing conversation patterns are the reference.
- Web: no code changes in this Mac-focused iteration.

## Non-goals
No inferred matter migration, board deletion, new storage schema, historical pane
control, automatic send, agent lifecycle changes, release or installation.
