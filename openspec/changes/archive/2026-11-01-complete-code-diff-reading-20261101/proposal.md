# Complete code and Diff reading

## Why
Expanded Chat and Trace calls can show an exhausted input preview as their only content. HTML reports lose local images on iOS, session typography has only three sizes, and a flat Diff is difficult to navigate on a phone. The user requested complete information, compact formatting and the supplied code/Diff card references.

## What Changes
- Read original selected tool input on demand through a verified source reference, without increasing ordinary transcript polling budgets or exposing results/context.
- Share compact offline code formatting and syntax colors; preserve original copying and unsupported source.
- Resolve local HTML image attributes through the existing parent-scoped document resource transport.
- Offer one-point session text-size adjustments with legacy migration.
- Share file/hunk Diff structure, counts, line numbers and independent folding across native and browser readers, preserving virtualization and complete source reading.

## Surfaces
| Surface | Change |
| --- | --- |
| CLI/server | Additive input references and authenticated on-demand read; HTML image evidence parsing; no new commands |
| Menu bar | No separate reader change; existing identity/focus behavior retained |
| iPhone/iPad | Full input, compact code cards, local HTML images, fine typography and structured Diff |
| Browser/mobile web | Same input contract and code/Diff models, touch targets and bounded hunk mounting |
| Website | No marketing, deployment or public-access changes |

## Design
Normal snapshots keep the existing 8 KiB/128 KiB preview budgets. The complete read verifies session/home, source position, call ordinal and original content hash. Missing, rewritten, oversized or incomplete records fail explicitly. Only known public input fields are selected. Native/web readers load this data on intent, keep retry visible and never copy a preview as a complete source. Formatting is presentation only.

HTML resource loading remains script-independent and offline: the immutable parent must declare the image, its type must be supported, and the source must remain within the parent's directory tree. Existing image budgets, script consent, guest restrictions and bridge isolation remain.

Diff describes the working tree rather than inferring a last turn. File and hunk counts derive from real hunk lines; structural Git headers stay in original source while the UI uses concise file/range labels. The native list stays virtualized and browser hunks mount in batches.

## Validation boundary
The native document fixture compiles current source and checks actual WebKit image loads on both simulators. React action tests cover reader/Diff controls. Simulator AX exposes one root and no targets, so no native tap, physical-device or VoiceOver acceptance is claimed. Shipping, installation and the already-submitted Store build are separate.
