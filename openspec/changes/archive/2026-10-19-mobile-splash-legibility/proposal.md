# Make the mobile launch identity legible and coherent

## Why

The light-mode RN splash places translucent white logo panes directly on a pale background; most of the mark disappears. Its low-contrast long subtitle and detached loader weaken the composition. The native launch screen currently shows a different dark word-and-cursor composition, despite the RN comment claiming parity.

## What changes

Keep the existing gtmux pane-grid motif and cyan accent. Give the mark a restrained dark plate with clearly visible neutral panes, tighten the wordmark and supporting copy, and group loading feedback intentionally. Share deterministic scale-aware artwork between the static native launch and RN splash so the identity does not jump during handoff. Use theme-aware launch backgrounds and readable supporting text. Native launch remains static; RN loading does not impose a minimum display delay and respects reduced motion. Preserve the app's startup/connection behavior.

## Surfaces

- terminal: no rendering or behavior change; core remains the shared source.
- menubar: no splash change; the existing Mac app is unaffected.
- phone: improve light/dark launch contrast and composition, safe-area placement and larger-text layout.
- iPad: use the same compact central identity with flexible surrounding space and orientation-safe loading feedback.
- Web: no change; the mobile launch component is not the Web client.

## Visual plan

A quiet terminal identity: an ink plate, a cyan focused pane and clear type, with useful space instead of faint shapes. The first view has one dominant mark, the name and one short supporting line; loading feedback sits in a coherent footer. The native screen is static and the RN handoff retains its anchor. Any loading motion is restrained, respects Reduce Motion and never delays readiness.
