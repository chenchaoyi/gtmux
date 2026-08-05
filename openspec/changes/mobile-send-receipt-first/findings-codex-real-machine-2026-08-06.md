# First-hand evidence — codex send on a real machine (2026-08-06)

> Status: EVIDENCE APPENDIX to `mobile-send-receipt-first`. Not a spec delta, not
> implemented, not archived. Recorded because it is real-machine confirmation of the
> failure modes this proposal exists to fix, plus a **second, separately-serious finding**
> (send clobbers an unsubmitted user draft) that had been an open question on the HQ board.
> Captured via a throwaway codex probe (`%31`, codex-cli 0.146.0, gpt-5.6-sol, cwd /tmp).
> No code was changed, no live user pane's draft was touched.

## Finding A — the "NOT delivered" verdict on codex is a false negative rooted POST-submit

A `gtmux spawn --agent codex` reported `✗ NOT delivered` while the goal was in fact fully
delivered and executed (correct result, single copy, no duplication). Reproduced with
`gtmux send`.

**Where it lives:** NOT `confirmPaste` (pre-submit). It is `Deliver`'s POST-submit
land-verification (`internal/dispatch/deliver.go`), shared by spawn and send. Proof: the
goal EXECUTED (so Enter was pressed / confirmPaste passed), and every failing verdict was
`judged_by:"screen"` — it fell to the screen-read fallback, not a receipt.

**Root cause (two compounding factors):**

1. **codex's receipt channel is dead.** Across a full experiment (send attempts, a task
   codex actually executed, a message codex queued) codex fired **zero** hook events:
   `gtmux events --since-seq` returned 0 for the pane, the paneless `native/` sink was
   empty, and `serve.log` shows no invocation — even though `~/.codex/hooks.json` correctly
   wires `UserPromptSubmit → gtmux hook`. So codex 0.146.0 does not invoke the hook at all.
   `Deliver` is `HookEquipped=true` for codex, waits `HookGrace`, gets no receipt, and
   falls to the screen-read.
2. **codex's composer renders with >1s latency, which the screen-scrape races.** Proven:
   `tmux send-keys -l "XQZZ"` was absent from an immediate capture but present after a 1.5s
   wait. Both `confirmPaste` (pre-submit) and the screen-read (post-submit) read stale
   frames. Nondeterministic by timing — this is the "时好时坏" the user reported.

**Three manifestations, one race resolving differently each time:**

| scenario | gtmux verdict | reality | classification |
|---|---|---|---|
| send → idle codex, pre-submit race LOST | `delivered:false` | not executed (paste judged fragment, C-u'd away, retried, gave up) | true non-delivery, **caused by gtmux clearing its own paste** |
| spawn/send → codex, pre-submit race WON, post-submit race LOST | `NOT delivered` | executed | **false negative** (the reported bug) |
| send → working codex (the #697 path) | `delivered:true` | queued + later ack'd (`PROBE2_MARK_9067_ACK 已收到`) | **correct, no false positive** |

**On false positives:** none observed this run (working-pane send reported true and was
truly delivered). But the mechanism exists: `pasteBusy` best-effort submits and reports
success; a best-effort submit that did not land (a different race) would false-positive —
the silent-loss window this proposal names.

**Why #697 does not fix this:** #697 fixed the working-pane *busy detection* (whole-frame
motion — the working-pane row above is correct because of it). It cannot fix the root,
which is the *receipt channel*, and codex's receipt is dead. The fix is this proposal's
core (receipt-first) **plus** making codex actually fire `UserPromptSubmit` — investigate
why the `~/.codex/hooks.json` wiring does not fire in codex 0.146.0 (Tier-1 event parity).

Open thread for the fix (not chased tonight): whether `spawn` delivers codex's initial goal
via a launch argument vs typing the composer — it affects where the land-verify should look.

## Finding B — `gtmux send` destroys an UNSUBMITTED user draft in the target pane

This answers an open HQ-board question ("does `gtmux send` swallow an unsubmitted draft in
the target pane? both answers matter, must be asked, don't guess"). The answer is **yes**,
and it is the worse of the two branches that were hypothesized.

**Code:** `Deliver`/`pasteWithGuard` do NOT check for a pre-existing user draft before the
first paste — a paste *appends* to whatever the box already holds (`deliver.go:443`), and
the only "leave it alone" check is whether the draft already holds *this delivery*
(`:250`), not unrelated user content. On a fragment verdict, `clearedForRetry` issues a
C-u, which clears the **whole line** (`:456`). Contrast the HQ-nudge path
(`internal/hqnudge`), which is safe *because* it guards: it reads the box with
`dispatch.DraftOf` first and, if a draft is present, queues its message instead of typing.
**The nudge guards; `gtmux send` does not.** The code is agent-agnostic, so Claude is
subject to the same risk (code-read only; no live Claude draft was touched) — only modulated
by render speed.

**Real-machine proof (on `%31`):** an unsubmitted draft `DRAFT_KEEP_31 用户正在打字还没发`
was placed in codex's composer. A `gtmux send` of `GTMUX_INJECT_9073 …` then produced:

```
› DRAFT_KEEP_31 用户正在打字还没发GTMUX_INJECT_9073 这是 gtmux 注入的测试消息,请忽略。
• 已忽略该测试消息。
```

i.e. gtmux appended its message to the user's draft and the concatenation was **submitted
as one prompt** (codex processed it and replied). The user's private, unfinished draft was
sent to the agent. And gtmux's own verdict was `delivered:false` (Finding A's false
negative) — so it **destroyed the draft AND believed it had failed**, which would prompt a
re-send and a repeat.

**Two destructive branches, both bad, selected only by the render race:**
- **(b2) concatenate-and-submit** — observed above: the race is won, Enter fires, and the
  user's draft is submitted glued to gtmux's message.
- **(b1) C-u wipe** — by code (`clearedForRetry`, `:456`): the race is lost, the fragment
  path C-u's the whole line, erasing the user's draft along with the paste.

**Impact:** every time HQ (or a user) drives a pane that already holds someone's half-typed
input, `gtmux send` can either send that half-typed input to the agent or erase it — with no
trace, and (on codex) while reporting failure.

## Implication (for discussion — not implemented tonight)

- Finding A ⇒ the receipt-first core of this proposal, plus fixing codex's hook firing.
- Finding B ⇒ a distinct, arguably higher-priority fix: **`gtmux send` must adopt the same
  pre-paste draft-guard the HQ-nudge already has** (read `DraftOf`; if the box holds a
  draft that is not ours, do not append/clear/submit — refuse or queue). This is orthogonal
  to receipt-first and applies to all agents.

## Boundary

No code change, no PR against implementation, no release. v0.45.6 is already out; whether to
hotfix is the commander's decision. This document is evidence only.
