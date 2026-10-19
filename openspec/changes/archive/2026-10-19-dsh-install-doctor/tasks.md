## Implementation

- [x] Share provider-declared plugin writers between install and doctor.
- [x] Check both DSH artifacts and distinguish installed files from runtime loading.
- [x] Detect DSH installations without enabling TUI radar detection.
- [x] Add failing regressions and validate repair/ownership boundaries.
- [x] Clarify installation selection and synchronize paired docs/current specs.

## Acceptance

Local regressions and CI are recorded in the PR. No real DSH profile or running Web process is changed by tests. File installation does not establish runtime loading. No new independent reviewer or live APNs/UI acceptance is claimed. Automatic selection is explicit; the existing Claude default is preserved.
