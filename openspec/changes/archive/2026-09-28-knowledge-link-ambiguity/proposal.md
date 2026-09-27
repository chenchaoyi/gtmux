# Detect ambiguous knowledge references

## Why

The knowledge linter accepted a bare wiki slug shared by entries in different
topics and attributed the link to the first entry in ledger order. This makes
knowledge navigation and orphan counts unreliable.

## What changes

- Resolve exact entry IDs before bare slugs.
- Report a bare slug with multiple live matches as `ambiguous-link`.
- Preserve exact references to superseded entries and resolve their successors.

## Boundary

The linter remains read only. Machine ledger links are corrected through
individual `knowledge supersede` operations after checking their targets.

## Surfaces

- **Terminal / 终端** — lint names ambiguous links and asks for a full ID.
- **Phone / 手机, iPad / 平板, Menubar / 菜单栏, Web / 网页** — no interface change.
