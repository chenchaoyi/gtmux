# Authoritative Radar read states

## Why

A connected stream can appear before the session list has loaded. The App currently shows zero sessions and a generic empty tutorial during that interval and on some failed reads.

## What Changes

Track the current client’s successful snapshot and bounded active read separately from connectivity, serialize burst refreshes with one trailing update, retain cached content, and show loading/error/real empty distinctly. Reuse the browser’s fixed short Dune excerpt only after a successful empty worker list. Preserve authorization, data ownership, existing activity/notification contracts and current polling cadence.

## Surfaces

- terminal: existing commands/contracts unchanged.
- menubar: existing consumer unchanged.
- phone: shared Radar explicit read states, actual refresh feedback and genuine empty excerpt.
- iPad: same RadarPanel/sidebar state contract and palette.
- Web: existing welcome/excerpt is the reference; no browser behavior change.
