# tasks — send-feedback-on-the-phone

Blocked on the commander's decision; nothing is implemented.

## 1. Decision 1 — a failed send says why

- [ ] 1.1 Decide the wording and placement (proposal: the failure bar carries the core's sentence).
- [ ] 1.2 Render the reason the client already keeps (`sendResult`).
- [ ] 1.3 "Send anyway" for `refused-draft` only, explicit about what it overrides.
- [ ] 1.4 Tests against the fake for all three refusals.

## 2. Decision 2 — queued vs delivered

- [ ] 2.1 Choose form A or B.
- [ ] 2.2 Form A only: the `queued` bit — contract, serve, spec delta.
- [ ] 2.3 Implement the chosen form + tests.
