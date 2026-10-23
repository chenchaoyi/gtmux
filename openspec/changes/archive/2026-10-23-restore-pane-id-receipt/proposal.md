## Why

Restore reports unnamed layout differences without a useful next step, and pane IDs
change after reboot without a visible correspondence. The backup-age sentence also
mixes machine units into awkward Chinese.

## What changes

- Name the backup time plainly and explain the cutoff on a separate line.
- Describe extra/missing panes and size/position changes, with an inspection action.
- Show a read-only per-window pane-ID receipt from the layout evidence already saved.
  Pair IDs only at identical layout positions or in single-pane windows; otherwise
  list the sets separately. Do not infer a process or conversation identity from it.

## Surfaces

- terminal: restore output changes; command arguments and plan JSON stay compatible.
- menubar: existing captured CLI output may include the receipt; no native UI changes.
- phone: no changes or new package requirement.
- iPad: no changes or new package requirement.
- Web: no changes.

## Core/API boundary

Read-only formatting uses existing layout metadata; no input, restore, cleanup or
API authority changes, and no new persistent state.
