# Compact access history and close audit attribution gaps

## Why
Access records use tall cards and a device-ID disclosure, making a short sequence difficult to scan. The native timestamp follows the system locale instead of the selected UI language. HQ archive exports share sampled knowledge reads; desktop conversations have no target in their read-coalescing key, and session creation omits its returned pane.

## What Changes
- Replace native/mobile cards with date-grouped log rows showing time, device, operation/target, HTTP outcome and inline metadata, without disclosures. Use selected-language dates and local 24-hour times. Preserve selection/copy, paging, retry and guest restrictions.
- Record each HQ archive export independently and report archive generation errors honestly; retain desktop session attribution and returned creation pane. Classify every authenticated route explicitly, with an inventory-derived regression guard.
- Keep access metadata allowlisted and additive; no payloads, credentials, arbitrary query values or historical reconstruction.

## Impact
Diagnostics and menu-bar/mobile specs; server/diag producers, native/mobile consumers, access API contract and paired English/Chinese documentation. Browser requests remain audited; no new browser viewer.

## Surfaces
- terminal / attach: CLI history includes desktop session targets; stream boundaries remain audited without keystrokes.
- menubar: compact local log window, inline metadata and selected-language dates.
- phone: compact virtualized rows, direct metadata, language-aware dates.
- iPad: same production screen, capped readable width.
- Web: browser API requests use improved attribution/classification; no new viewer or public/static-page audit.
