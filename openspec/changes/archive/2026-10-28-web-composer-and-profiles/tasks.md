## Implementation
- [x] Inspect iPhone/iPad composition and identity components and backend contracts.
- [x] Align shared composer controls, mobile layout, attachments, editor and recall.
- [x] Remove Quote and writable badges; add settings icon and identity sheets.
- [x] Verify sending, failures, state isolation, keyboard/mobile interaction and rendered flows.
- [x] Sync capability specs, documentation and archive the completed change.

## Acceptance
Use synthetic sessions and files in tests and preview. Do not send to installed agents. Native-specific keyboard accessory APIs are implemented using browser viewport and safe-area behavior rather than claimed as native API parity.

Verified: 60 browser logic tests and 14 rendered Chromium flows, including touch layout, pending-message reconciliation, retry identifiers, annotation flattening, staged attachments, recall, historical profiles and revocation. The historical-profile/revocation flow was rerun after strengthening those checks. Scoped Go Web tests pass. Desktop and 390px visual inspection used synthetic sessions on loopback. A physical iPhone/iPad keyboard and Safari were not exercised; the installed app and real agent sessions were not changed.
