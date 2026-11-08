## Implementation
- [x] Protect transitions and current questions.
- [x] Persist observations and fenced assessment receipts.
- [x] Wire bounded source ingestion, CLI processing and HQ reminders.
- [x] Verify isolated recovery/compatibility and core gates.

## Acceptance
Device UI, deployment and historical cleanup are outside phase 1.

Core make check, affected race tests, isolated restart/receipt cases, negative guard mutations and source-only design gate passed. Cgo-free CLI build/help smoke passed. Private exact-term policy was unavailable locally. Independent review, hosted CI, deployment and device acceptance are not claimed here.
