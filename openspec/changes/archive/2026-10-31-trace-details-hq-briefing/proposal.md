# Trace details and HQ briefing cards

## Why
Trace gives a generic heading more weight than the session name and repeats the last call under an expanded group. A horizontal input region makes long calls hard to read on a phone. HQ resources sit after the entire overview and require opening a reader to discover their values; grouped row lists obscure individual matters.

## What changes
Use the selected session title, wrap complete recorded tool input in Trace, and remove misleading group previews. Place real board, knowledge and usage summaries before matters. Present matters as compact individual briefing cards with current state, recorded preview or explicitly labelled current assessment, and a dated byline. Reuse existing models, APIs, readers and permissions.

## Surfaces
- terminal: no change; no CLI or schema changes.
- menubar: no code change; shared matter classifications, states and resource meanings remain the reference.
- phone: session-led Trace header, readable input details, top summaries and individual HQ briefing cards.
- iPad: same components; layouts use actual available width and text scale, with current matters flowing in one priority-ordered grid.
- Web: no code change; existing session-led Trace details and shared transcript data remain compatible.

## Boundaries
Owner-only resource reads are independent, serialized and scoped to the current client; guest and legacy fallback paths do not poll them. Console transitions cancel overview reads. Transient failures retain labelled readings; authorization failure clears private content. No new sends, follow state, approvals, telemetry or release are implied. Native renders, React actions, actual native interaction and VoiceOver evidence are recorded separately.
