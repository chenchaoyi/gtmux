## Implementation
- [x] Extract sidebar/composer components and share explicit matter grouping/status.
- [x] Add structured Overview and secondary, lossless handoff snapshot navigation.
- [x] Add independent sidebar disclosures and branded initial/refresh states.
- [x] Align shared conversation bubbles, copy feedback and time separators with mobile.
- [x] Add behavioral regression tests and verify they detect representative regressions.
- [x] Update paired documentation, run required checks and archive this change.

## Acceptance
Use a synthetic, read-only isolated native preview for compact/wide layouts, navigation,
copy and loading states when the UI channel is available. Production HQ, personal
records and agent clients remain untouched. CUA verified the rendered dark fixture, independent sidebar collapse across navigation,
copy confirmation/reset, gap separator, seam plus public text, branded cold resource
loading, and a lossless folded snapshot with plain pane references. Native-host tests
cover compact/wide layouts in both appearances. Actual VoiceOver speech and production
HQ interaction were not exercised. AX inspection is not speech acceptance.
