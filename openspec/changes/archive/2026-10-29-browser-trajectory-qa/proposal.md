# Make transcript traces useful across agents and sessions

## Why
The browser trace index omits already-recorded tool input, loses event identity on history changes, and makes narrow panels hard to read. Readable external sessions are mislabeled and have no trace entry. The user requested detailed testing against DSH web, mobile browser acceptance, and an App design assessment.

## What Changes
- Preserve current-main DSH, permission, terminal-key and interaction behavior while integrating the earlier browser workspace improvements.
- Index only public transcript data with stable event identities, explicit session/agent/turn context, recorded tool input, truncation indicators and clear evidence limits.
- Keep selection, scrolling and collapsed details stable during updates. Provide a compact drill-in detail view and keyboard/touch navigation.
- Offer read-only Chat/Trace for readable external sessions, keeping all input and terminal capabilities tied to real authorized panes.
- Read owner agent profiles from the existing recorded-facts endpoint, using message session/time for historical avatars and retiring closed or stale reads.
- Exercise providers, history seams, empty/long/changing data, session switches, access failures, appearance and mobile layouts. Document App feasibility separately from delivered functionality.

## Surfaces
- terminal: no new commands; existing public transcript and input contracts remain authoritative.
- menubar: preserve the earlier local workspace entry and current-main restore behavior.
- phone: mobile browser trace details use a focused reading view; native App trace is a documented proposal, not a shipped feature.
- iPad: browser trace adapts to available width; native App split-view behavior is assessed separately.
- Web: improve the ordered trace index, external readers, responsive layout and cross-provider acceptance.
