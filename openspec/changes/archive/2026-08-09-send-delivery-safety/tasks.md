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

## 4. Scope + robustness (the follow-up round, same day)

- [x] 4.0a `Opts.HasComposer` gates the guard to panes a KNOWN agent drives
      (`dispatchbridge.knownAgent` off the registry). A vim/ssh/TUI pane has no composer,
      and the no-box degrade path returns its TRANSCRIPT as a draft — measured live at 347
      chars on a pane running `diting`. Off by default, so an unconsidered caller gets
      pre-guard behavior. `send.go`'s unverified path now builds its opts through
      `DeliverOpts` so the two paths cannot drift.
- [x] 4.0b The guard FAILS OPEN on everything it cannot judge: unreadable capture (tmux
      failed / pane gone / server wedged), copy-mode (the screen is scrollback, not the
      composer), no locatable input region, our own text. Its job is to protect a send,
      never to block one.
- [x] 4.0c BOUNDED: exactly 2 reads + 1 poll interval on the refusal path, 1 read and no
      wait on the common (empty-box) path; no loop, no retry; nil-Sleep tolerated.
      Pinned by `TestDraftGuardIsBounded`.
- [x] 4.0d No call site swallows a refusal — `deliverHQBriefing` reported its result
      instead of discarding it.
- [x] 4.0e Cost measured on live panes rather than assumed: the guard is ONE extra
      `capture-pane` ≈ 6.6 ms (process spawn, not payload — a visible-only capture is
      5.8 ms), and `DraftOfColored` parses 16 KB in 73 µs, so the parse is 1–3 % of it.

## 4. Consistency
- [x] 4.1 `openspec/specs/agent-dispatch/spec.md` synced (1 ADDED, 2 MODIFIED + scenarios).
- [x] 4.2 `docs/cli.md` `gtmux send` (with the fail-open decision table), the CLAUDE.md
      dispatch paragraph, and the TROUBLESHOOTING entries for BOTH false positives
      (ghost text; no-composer panes) updated.
- [x] 4.3 Archived with the implementing PR.
