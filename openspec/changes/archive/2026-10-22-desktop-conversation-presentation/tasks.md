## Implementation

- [x] Align the native conversation hierarchy and participant attribution.
- [x] Preserve and inspect structured tool input with operation icons and bounds.
- [x] Reuse native reading, avatar, copy and tail-positioning primitives.
- [x] Validate synthetic bilingual light/dark layouts, actions and lifecycle regressions.
- [x] Update the paired design guide and synchronize the capability specification.

## Acceptance

Synthetic fixtures do not establish real-device, spoken VoiceOver or independent
review acceptance. No release or replacement of the installed application is part
of this change.

Local acceptance: 12 focused native tests pass, including in-process AX presses for
folding/tool inspection across eight language/appearance/width combinations, public
HQ attribution, structured evidence, serial reads and historical scroll preservation.
Synthetic PNGs were inspected; this is author validation, not independent review.
