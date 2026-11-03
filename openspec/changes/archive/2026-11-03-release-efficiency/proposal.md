## Why

Release 169 exposed preventable release overhead: Android notes were checked only after a second CI round, phone and tablet emulators ran serially, and transient artifact-download failures required ad hoc recovery scripts. Runner acquisition also failed twice; that is distinct from compilation time and does not justify weaker gates or an unverified runner migration.

## What Changes

- Allow stamping a reviewed future version without creating a temporary tag; add a read-only, explicit-platform preflight before the first release PR.
- Build the Android QA package once, transfer its exact-source checksum receipt, and run the complete phone and tablet suites independently in parallel. Both remain mandatory.
- Provide one bounded, authenticated encrypted-artifact retrieval command for both platforms, with pinned source/run/attempt/digest, safe redirects, checked cache reuse and private output.
- Document timing categories, recovery boundaries and the evidence needed before changing runner architecture. Keep release publication, signing, protected environments and latest-main CI checks intact.

## Surfaces

- terminal: maintainer-only preparation and retrieval commands; normal gtmux CLI unchanged.
- menubar: no product UI or signing changes.
- Phone: unchanged product behavior; complete Android phone regression remains required.
- iPad: unchanged product behavior and native acceptance boundaries.
- Web: unchanged product behavior and browser regression gates.

## Non-goals

No new release, installation, Store/Play submission, permission change, runner registration, signing change, automatic mutation retry, or cleanup of historical/private delivery archives.
