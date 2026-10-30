# Mobile HQ matter overview

## Why

The phone's HQ page still emphasizes a digest and global console, while the menu bar organizes persistent work into explicit matters. Add a mobile landing page over the same workspace records: needs input, following, recent changes, and a readable matter sheet with participants and updates. Keep the existing HQ console and resource readers accessible.

## What Changes

Expose a read-only, owner/full-device HTTP adapter to the existing workspace list/detail builders. No mutation, follow, approval, dispatch or delivery behavior changes. Older servers retain the existing HQ page.

Trace exploration also fixes modified-key interception and repeated clockless prompt identity across history expansion. Parsers supply stable opaque source positions; both readers use the shared trace model.

## Surfaces

- terminal: additive opaque transcript record identity; local HQ reads keep their semantics.
- menubar: reference implementation; workspace protocol and UI unchanged.
- phone: matter overview and bounded detail over owner-only shared reads, existing console retained.
- iPad: the same components adapt to available main-pane width.
- web: Trace modifier shortcuts and ambiguous historical record identity fixed in the shared model; no new HQ workspace UI.
