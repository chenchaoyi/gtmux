# Full-screen chat controls clear device corners

## Why
Full-screen removes the top safe-area edge for reading, but the fixed top-right Collapse all control inherits that removal. Its label can sit under rounded corners or the Dynamic Island.

## What changes
Position full-screen chat fold controls independently using the device top inset, with an opaque backdrop and horizontal gutter. Keep normal-mode flow, horizontal safe-area protection, scrollable full-screen content and both fold actions.

## Surfaces
- phone and iPad: shared DetailView and ChatView, including landscape/split layouts.
- terminal, menubar and Web: no UI behaviour change.

## Validation
Rendered ChatView regression covers normal mode, portrait top inset and landscape zero top inset, including collapse/expand actions. Physical device visual acceptance remains pending; no release or installation in this batch.
