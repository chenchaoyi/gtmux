# Filter and aggregate recorded access activity

## Why
A chronological log alone makes device-specific investigations cumbersome. Full IDs must distinguish same-name devices, and aggregation must not claim sampled reads are request totals.

## What Changes
Add exact device/actor, operation and outcome filters to the shared bounded reader and CLI/API. Add device grouping with recorded-activity/outcome counts and first/last times, capped by the existing scan budget with continuation. Bind new cursors to filter/view context. Native/mobile offer Logs and Devices, immediate filters and group-to-log navigation; incomplete summaries remain explicit.

## Surfaces
- terminal / attach: shared logs flags and grouped JSON/text, no stream changes.
- menubar: native compact filters and device summaries.
- phone: existing anchored menus and shared list, no new navigation stack.
- iPad: same controls and readable width.
- Web: additive owner API, no new viewer.
