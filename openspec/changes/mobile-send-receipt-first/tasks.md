# Tasks — mobile-send-receipt-first

Status: PROPOSED (not started), and DEPRIORITIZED 2026-08-11 — the symptom it was
written against has been absent since #692 / #697 / #703 closed three specific scrape
defects (serve.log through 2026-08-10 has zero send failures). The architecture is
unchanged, so this is now insurance against the next agent-version drift rather than a
live fix. See the proposal's "Evidence update" section before scheduling it.

Sequenced so each slice ships value on its own — §1 is the standalone hedge if only one
slice is ever done.

## 1. Demote the scrape to a pre-check; stop hard-failing a working pane

- [ ] 1.1 `POST /api/send` (`internal/app/serve.go`): when the pre-submit guard cannot
      confirm on a pane the radar reports WORKING, submit best-effort and return a
      non-error "pending" outcome instead of "not confirmed".
- [ ] 1.2 Keep the fast happy path (confirmed draft ⇒ immediate "sent") unchanged.
- [ ] 1.3 Tests: a working pane whose draft never reads back does NOT return an HTTP error;
      a quiet pane with a genuine fragment still does.

## 2. Receipt reconcile (the deterministic channel)

- [ ] 2.1 After an optimistic submit, arm a receipt watch on the `UserPromptSubmit` event
      matched by prompt head (`MarkAwaited` already exists; extend it to feed a
      confirm/expire signal the phone can read).
- [ ] 2.2 Confirm on a matching receipt within a grace window; expire to
      "could-not-confirm" otherwise.
- [ ] 2.3 Tests: receipt-lands ⇒ confirmed; no-receipt ⇒ expired; wrong-head receipt does
      not confirm.

## 3. Queue-until-idle safety net

- [ ] 3.1 Hold a message that could not be confirmed against a working pane; re-deliver via
      the robust idle path on the radar working→idle transition.
- [ ] 3.2 Idempotency: a held message carries a stable id so retry/reconnect never
      double-delivers (reuse the `sendID` interlock).
- [ ] 3.3 Tests: held → pane goes idle → delivered exactly once; pane never idles within a
      cap → surfaced, not silently dropped.

## 4. Mobile UX

- [ ] 4.1 Render three states in the composer/transcript: sent · pending · could-not-confirm
      (replace the synchronous red banner as the only outcome). See `mobileapp/src/ui/SendFailedBar.tsx`.
- [ ] 4.2 Reconcile a pending send when its message appears in the transcript / its receipt
      arrives; no duplicate rows.
- [ ] 4.3 jest coverage for the state transitions.

## 5. Per-agent receipt parity (Tier-1)

- [ ] 5.1 Audit each hook-equipped agent's `UserPromptSubmit` receipt reliability
      (`internal/driver`, `internal/agents`); Codex first, since it triggered this.
- [ ] 5.2 Where a receipt is missing/unreliable, fix it in the agent's driver/manifest —
      not by adding another TUI special-case to the scrape.

## 6. Consistency (per the repo rule)

- [ ] 6.1 Update `openspec/specs/agent-dispatch/spec.md` (sync this change's delta) and
      any doc/`CLAUDE.md` line describing `POST /api/send` confirmation.
- [ ] 6.2 `api/contract.md` if the `/api/send` response shape gains a pending id.
- [ ] 6.3 Archive this change once implemented.
