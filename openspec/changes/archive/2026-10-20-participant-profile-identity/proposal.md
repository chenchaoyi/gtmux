## Why

Sparse participant profiles leave a large empty dialog, provider identifiers are
shown with raw casing, and the HQ sender profile explains neither its role nor
the agent currently fulfilling it. The global fold icon also hides a previously
clear text action.

## What Changes

- Fit profile dialogs to their content, retaining bounded scrolling and an explicit loading placeholder.
- Format known provider brands only in presentation; preserve custom identifiers and raw facts.
- Explain HQ as the coordinating agent, distinguish recorded sender identity from the current supervisor, and use only the same connected server's radar for current identity.
- Restore the brand-coloured Collapse all / Expand all text action.

## Surfaces

- terminal: no lifecycle, dispatch, or profile collection changes.
- menubar: no changes; these dialogs belong to the mobile conversation.
- phone: compact profiles, HQ role and scoped identity, explicit global folding action.
- iPad: the same shared components with height limits and scalable text.
- Web: no change to transcript presentation or API authority.

## Non-goals

No historical identity backfill, extra profile polling, arbitrary provider title-casing,
new agent integration, or release/access-policy change.
