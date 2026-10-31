# Refine reader navigation and adaptive layouts

## Why
Trace details and their parent used identical Close icons. iOS removed the painted
reader during native dismissal, exposing a white backdrop. Exploratory testing also
found responsive HTML image rewriting and large-text Diff sizing defects. iPad
multitasking must preserve the selected source without a separate reader implementation.

## What Changes
- Distinguish record Back/Hide details from the outer Trace Close, retaining the themed tree through dismissal while honoring scope retirement.
- Share foremost-overlay Escape handling between readers, profiles and anchored menus.
- Keep Diff gutters and wrapped content within the measured canvas at large text sizes.
- Preserve quoted/unquoted HTML srcset attributes and declared image cache queries.

## Surfaces
| Surface | Change |
| --- | --- |
| CLI/server | Declared HTML images accept file cache queries under the same immutable-parent and directory checks. |
| Menu bar | No changed reader; existing focus and identity behavior retained. |
| iPhone/iPad | Shared navigation, dismissal, keyboard ownership and adaptive Diff layout; no copied iPad screen. |
| Browser/mobile web | Distinct Trace detail navigation and resilient resize sequence; six-digit Diff gutters. |
| Website | No deployment or marketing changes. |

## Verification
Component actions cover dismissal lifecycle, scope changes, resize and keyboard
ownership. Rendered browser tests cover multiple agents, history, folding, full input,
Diff and phone/tablet widths. The current native document fixture verifies real WebKit
image loads on both simulators. Hardware keyboard dispatch, physical-device UI and
VoiceOver speech remain separate acceptance evidence.
