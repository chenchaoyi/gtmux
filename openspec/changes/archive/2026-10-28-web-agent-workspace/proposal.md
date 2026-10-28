# Browser agent workspace

## Why

The browser conversation surfaces had diverged from the app and could not serve as a complete local management window.

## What Changes

Make the browser a local and remote agent workspace with consistent conversation rendering, input, and an inspectable transcript trace. Add a menu-bar entry that opens an authenticated loopback page without enabling remote access.

The trace uses the existing parsed transcript: prompts, chronological assistant segments, and tool summaries. It does not claim model request timing, tokens, raw arguments, or tool output that the transcript contract does not expose.

## Surfaces

- terminal (incl. attach): no CLI or attach contract changes; existing authenticated send and transcript APIs are reused.
- menubar: add a local workspace footer action and manage only its own loopback serve child.
- phone: no implementation changes; preserve its existing conversation behavior and permission model.
- iPad: no implementation changes; preserve the regular native shell.
- Web: unify conversation renderers and input, add transcript trace, improve readable spacing and copy affordances at desktop and narrow widths.
