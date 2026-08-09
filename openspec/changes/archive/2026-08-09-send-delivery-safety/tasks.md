# Tasks — send-delivery-safety

Status: COMPLETE — implemented, spec'd and archived in one PR (the three defects were
found by review, not in flight).

## 1. The draft guard
- [x] 1.1 `draftBlocked` + `StateRefusedDraft`; wired into `Deliver` AND `PasteAndSubmit`.
- [x] 1.2 `Opts.ClobberDraft` kept separate from `Opts.Force`; `gtmux send --force` sets
      both, serve's per-send Force sets neither.
- [x] 1.3 Callers report it: CLI (en+zh) and `POST /api/send` ("that pane has unsent text").
- [x] 1.4 Tests: refuses + writes nothing + quotes the draft; our own draft is not a
      clobber; ClobberDraft overrides; **interlock Force alone does NOT** (the phone-safety
      property); the unverified path refuses too.

## 2. A failed send may be retried
- [x] 2.1 `IO.ForgetSend` + `dispatch.ForgetSend`, called on every `failed` return.
- [x] 2.2 Tests: failed drops the record; `queued` keeps it.

## 3. A placed draft is not destroyed
- [x] 3.1 `confirmPaste` reports whether anything reached the draft; a hook-equipped
      fragment-with-placement submits instead of clearing.
- [x] 3.2 Tests: an idle pane's late render is neither cleared nor re-pasted and lands via
      the receipt; the hook-less path still clears and retries.

## 4. Consistency
- [x] 4.1 `openspec/specs/agent-dispatch/spec.md` synced (1 ADDED, 2 MODIFIED + scenarios).
- [x] 4.2 `docs/cli.md` `gtmux send` and the CLAUDE.md dispatch paragraph updated.
- [x] 4.3 Archived with the implementing PR.
