## Implementation

- [x] Replace heavy activity cards with compact reader-controlled disclosure.
- [x] Keep original-input scrolling/copy and avoid empty argument inspectors.
- [x] Place reply actions next to their final text.
- [x] Separate historical facts from on-demand same-session launch observations.
- [x] Distinguish initial loading, failure/retry and unrecorded profile data.
- [x] Complete behavioral regressions.
- [x] Align bilingual documentation, API notes and capability specs.

## Acceptance

Author regressions and simulator fixtures are not independent review or physical-device acceptance. Preserve any actual UI-tool and VoiceOver gaps in delivery.

Normal-size iPhone simulator checks used an existing Release native shell with current JS and isolated synthetic data. New native compilation, this iteration’s iPad/large-type layouts, Android, physical-device and VoiceOver speech/focus acceptance were not performed. UI-driver settling/focus failures were re-read and excluded from passes.
