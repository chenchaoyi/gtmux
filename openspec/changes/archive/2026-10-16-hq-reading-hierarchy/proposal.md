# Readable HQ typography and meaningful activity icons

## Why
The full HQ workspace reuses 9.5–13pt popover typography: sidebar group headings are smaller and lighter than their children, reading bodies are small, and activity rows share one clock glyph regardless of meaning. Phone Settings also reuses a server glyph for connection status, switching hosts and a local archive copy.

## What Changes
Add a scoped HQ reading type scale for navigation, body, Markdown, resources and native input. Classify existing action kinds, use semantic SF Symbols and a category filter without changing chronology or audit data. Give phone settings distinct existing icons for their meanings. Enlarge composer symbol keys and meet the 44pt touch-target minimum.

## Surfaces
- Terminal: unchanged; action storage and CLI kinds are unchanged.
- Menu bar: the full HQ window adopts the reading scale and classified activity; the compact menu popover retains its own typography.
- Phone: settings icons distinguish status, server selection and local records; existing labels/actions remain. Composer symbol keys gain readable glyphs and 44pt targets.
- iPad: the same settings component receives the icon and composer key fixes; its HQ interface remains distinct from the native workspace.
- Web: unchanged; no new activity API or web typography changes.

## Boundaries
Unknown action kinds remain visible; category never implies current running/success state. Existing retention bounds and source separation remain. Native UI tool availability is reported separately from isolated render/test results. No release or installation in this task.
