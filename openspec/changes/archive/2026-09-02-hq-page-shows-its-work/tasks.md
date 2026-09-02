# Tasks — hq-page-shows-its-work

## 1. Header: the judgment, and everything else behind a disclosure

- [x] 1.1 `hqHeaderModel.ts` (pure — the name avoids the macOS case-insensitive collision with `HQHeader.tsx`): pick HQ's own latest `⟣` brief from the turns; decide what the disclosure holds; resource promoted only at the red tier
- [x] 1.2 `HQHeader.tsx`: back + name + dot + verdict line + disclosure, composed from the pure model
- [x] 1.3 Tests: the brief is HQ's own words when it has any and the counts otherwise; a red tier reaches the standing line; a green one does not
- [x] 1.4 HQScreen composes the header instead of inlining it

## 2. The middle zone becomes the supervisor's own work

- [x] 2.1 `hqActsModel.ts` (pure) + the core-side `?acts=1` filter (the client-only plan died on measurement — 200 records span 3.9h): `gtmux:audit:*` → {when, verb, target, detail, outcome}; the week tally; the supervisor/fleet filter
- [x] 2.2 `HQActs.tsx`: the tally strip, the day-grouped act rows, the filter, an empty state in words
- [x] 2.3 Tests: every audit kind gets a phrase (a new kind must not render as a raw token); the tally counts a week; the filter partitions without losing records
- [x] 2.4 The zone's selector signal reports unseen acts, not just fleet events

## 3. The conversation shows the work as it happens

- [x] 3.1 Stream the in-flight turn's steps; the timer stays as the fallback when there is nothing yet
- [x] 3.2 The current turn's steps are expanded, history collapsed
- [x] 3.3 Tests: an in-flight turn with steps shows them; with none it shows the timer; a finished turn collapses again

## 4. Close it out

- [x] 4.1 `npm run check` green; `check-design.sh` green
- [x] 4.2 MOBILE.md §17 rewritten to the new page (it is the design authority — a page that no longer matches it is drift)
- [x] 4.3 Sync specs + archive
