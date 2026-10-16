# Keep terminal reading position while changing layout

## Why
Changing Wrap/Original rebuilds the row grid immediately without painted feedback. The old restore waits only for a content-height event, and intermediate native scroll callbacks can mistake a clamped history position for following the tail. Fraction-only anchors lose the place within a long row on a round trip.

## What Changes
Show the shared loading mark before row reflow, retain stable scrollers, preserve a captured line/cell and intra-row offset, and restore after commit even when height does not change. Keep following readers at the live tail; cancel superseded work and protect active selection.

## Surfaces
- Terminal: CLI/tmux output unchanged; this is client presentation.
- Menu bar: unchanged; the native HQ workspace does not expose this width control.
- Phone: Terminal Wrap/Original gains feedback and anchored restoration.
- iPad: shared renderer receives the same behavior in regular/fullscreen layouts.
- Web: uses a separate xterm fitting model and has no corresponding Wrap/Original switch; unchanged.

## Boundaries
No server resize, transcript reconstruction, native selection API or new dependency. Actual device timing and VoiceOver remain distinct from component tests.
