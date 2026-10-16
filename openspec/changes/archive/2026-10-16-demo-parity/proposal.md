# Keep the offline demonstration aligned with the product

## Why
The real phone/iPad shells are shared, but sample data has drifted: rejecting a permission runs tests, HQ retains the old waiting assessment, the shared composer calls an excluded sendResult method, usage names other panes/agents and omits history, and knowledge includes obsolete installation advice. Codex segmented replies and public desktop conversations are absent from the tour.

## What Changes
Keep one isolated, disposable sample world. Make approval outcomes and all derived views agree, demonstrate current read-only Codex conversations and intermediate replies, and supply coherent usage/history and knowledge samples. Keep real host administration unavailable.

## Surfaces
- Terminal: unchanged; no real pane input or CLI behavior changes.
- Menu bar: unchanged; its production HQ workspace is not an offline demo server.
- Phone: current production views over corrected synthetic data, including read-only Codex history.
- iPad: same data and shared views in the regular shell.
- Web: unchanged production UI; the screenshot test server reuses these sample fixtures and verifies HTTP parity.

## Boundaries
No deployed public demo service, real pairing, push, route switching, access history, or real session creation. No native build/device acceptance is implied by unit and fixture checks.
