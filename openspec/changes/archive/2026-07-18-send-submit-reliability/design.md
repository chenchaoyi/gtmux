# Design: send-submit-reliability

## Context

Three code paths type text into a pane, all built on `tmux.Paste` (`load-buffer`
+ `paste-buffer -p -d`) followed by a *separate* `SendKey "Enter"`:

| Path | Code | Confirms before Enter? | Verifies landed? |
|---|---|---|---|
| `gtmux spawn` / `gtmux send` (default) | `dispatch.Deliver` | head-only (`ContainsHead`) | yes |
| `POST /api/send` (phone / menu-bar) | `app.sendToPane` | **no** — blind Enter | no |
| `gtmux send --no-verify` / `--no-enter` | `app.cmdSend` plain path | **no** — blind Enter | no |

The shared weak link is the "did the paste land?" predicate `draftHasDelivery` →
`ContainsHead(draft, text)`, which matches on `NormalizeHead` = the first
`headRunes` (40) runes only. Consequences:

1. **Truncation reported as success.** For a payload longer than its head, the
   head can render a frame before the tail. `confirmPaste` returns `pasteInDraft`
   on the head; `Deliver` fires Enter; a half-rendered draft submits; the loop
   then finds the same 40-rune head in *history* and returns `StateLanded`.
2. **Conditional bracketing.** `paste-buffer -p` brackets only when the app has
   DECSET 2004 on *at that instant* (off during a startup gate, a modal, or a
   redraw window). Raw newlines then submit line-by-line, and an unterminated
   `ESC[200~` can leave the composer treating later Enters as inserted newlines —
   the "two manual Enters never submitted" symptom.
3. **Unverified paths don't check at all** — `sendToPane` pastes and immediately
   sends Enter with zero settle and zero content check.

## Goals / Non-Goals

**Goals**
- The pre-submit check confirms the FULL payload (head **and** tail), so a
  fragment is never submitted as a whole task.
- No submit path leaves an unterminated bracketed-paste state.
- The unverified paths gain a cheap pre-submit content check (no post-submit
  landed verification — they stay fast).
- Re-submit (swallowed-Enter retry) re-confirms full content before each Enter.

**Non-Goals**
- Not changing the landed-verification budget, hook-first layering, re-send
  interlock, or the queued-message detection.
- No new flags / HTTP fields.

## Decisions

### D1 — Full-payload confirmation: head + tail fingerprint

Add `NormalizeTail(s)` (mirror of `NormalizeHead`: the LAST `headRunes` runes of
the space-normalized text) and `ContainsTail`. `draftHasDelivery(draft, text)`
becomes:

```
full == (ContainsHead(draft, text) && ContainsTail(draft, text)) || looksCollapsedPaste(draft)
```

- For text ≤ `headRunes`, head and tail overlap → head match already implies the
  whole thing; `ContainsTail` is a no-op (tail == head).
- A collapsed-paste placeholder (`[Pasted text +N lines]`) still counts — the full
  content is folded and unreadable on screen, so the placeholder is the evidence.
- Because `confirmPaste` only returns `pasteFragment` after the full settle window
  with no full-match, a slow tail is *waited out*, not submitted.

This alone fixes truncation on the verified path and is the primitive the other
paths reuse.

### D2 — Atomic bracketed delivery (no lingering paste state)

Keep `load-buffer` + `paste-buffer -p -d` as the primitive (it writes the whole
buffer in one pty write; `-p` brackets when the app asked). The robustness comes
from D1 + D3, not from a second paste mechanism: **we never send Enter until the
draft is confirmed to hold the full payload**, so a raw-newline paste that
submitted a line early is caught as a fragment (draft no longer holds the tail →
retry per the existing no-duplicate rules), and an unterminated-paste composer is
detected as "draft doesn't match" rather than blindly Entered into. We do NOT try
to force bracketing with a hand-rolled `send-keys -l $'\e[200~…\e[201~'` — that is
the `send-keys -l` mid-TUI "not in a mode" failure the paste-buffer path was
introduced to avoid. The atomicity guarantee is *behavioral* (confirm-before-
submit), which is agent-agnostic.

### D3 — Pre-submit confirmation on the unverified paths

Factor the paste→confirm-full→Enter core out of `Deliver` into a small helper
usable without the landed-verification loop:

```
PasteAndSubmit(io, opts, text) — paste (with the existing fragment guard),
  wait up to the settle window for the draft to hold the FULL payload,
  then send Enter once. Returns whether the full draft was confirmed.
```

- `Deliver` keeps calling `pasteWithGuard` + its verify loop (unchanged control
  flow); `pasteWithGuard`/`confirmPaste` now use the D1 full-match predicate.
- `sendToPane` (POST /api/send) and `cmdSend`'s plain path call `PasteAndSubmit`
  so they confirm the draft before Enter. If the confirm times out they still
  send Enter (best-effort — these paths never promised verification) but the
  common case no longer races. Latency: a healthy paste confirms in ≤1 frame, so
  the phone path stays fast; the settle window only bites a genuinely slow render.

### D4 — Re-Enter only on a confirmed full-draft match

In `Deliver`'s verify loop the swallowed-Enter branch currently fires on
`inDraft && prevInDraft` where `inDraft = draftHasDelivery(...)`. With D1 that
predicate is already full-match, so "blind re-Enter" is fixed by construction:
a draft that has been submitted (empty) or mangled no longer satisfies the
full-match, so no further Enter is sent. Make the intent explicit in the code
comment and pin it with a scenario.

## Risks

- **A legitimately collapsed paste** whose tail is folded away: covered — the
  placeholder branch of `draftHasDelivery` short-circuits the tail check.
- **Tail fingerprint vs TUI re-wrap**: `normalizeSpace` already collapses
  whitespace before matching, so a re-wrapped tail still compares equal (same
  reasoning `ContainsHead` relies on).
- **Very short payloads**: head==tail, so no behavior change.

## Migration

Pure internal behavior change. No config, no wire-contract, no state-path change.
Existing dispatch tests that asserted head-only landing get a tail added to their
fixtures; new scenarios pin truncation-not-submitted and unverified-path confirm.
