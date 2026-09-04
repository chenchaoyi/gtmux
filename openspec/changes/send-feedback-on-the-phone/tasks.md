# tasks — send-feedback-on-the-phone

Decided and implemented (both decisions), 2026-09-04.

## 1. Decision 1 — a failed send says why

- [x] 1.1 Decide the wording and placement (proposal: the failure bar carries the core's sentence).
- [x] 1.2 Render the reason the client already keeps (`sendResult`).
- [x] 1.3 No override: `POST /api/send` has no field to carry one (the core's
      ClobberDraft is reachable only from the CLI), so the copy says what is true and
      what the reader can do instead. Adding the field would be a contract change, not
      a display change — recorded here rather than smuggled in.
- [x] 1.4 Tests against the fake for all three refusals.

## 2. Decision 2 — queued vs delivered

- [x] 2.1 Form B chosen (client-side, read off the target's status; no API change).
- [ ] 2.2 Form A (server reads the screen for a `queued` bit) — NOT done, and not needed
      unless a precise receipt is ever wanted.
- [x] 2.3 Implement the chosen form + tests.
