# send-delivery-safety — a delivery never destroys what it did not write

Origin: the 2026-08-09 backlog review. Three defects in ONE code path
(`internal/dispatch/deliver.go`), each of which loses a message or a user's words, and
none of which had a ticket. Implemented and archived together because they are the same
mistake in three places: **the delivery path treated the pane as its own**.

## Why

1. **A send eats an unsubmitted draft.** A paste APPENDS to the input box. `Deliver` went
   from the re-send interlock straight to `pasteWithGuard` without ever reading what was
   already in the box, so a payload delivered into a pane whose owner was mid-sentence was
   concatenated onto their words and submitted as one message — their line mangled and sent
   without them pressing Enter. The NUDGE channel has refused a non-empty box since
   `hq-nudge-hardening`; the dispatch channel never did, so the CLI, HQ, and the phone could
   all do this. The phone is the worst case: a send from another device into a pane whose
   owner is typing, with nobody present to undo it.

2. **A failed send poisoned its own retry.** The interlock records the payload when the
   paste is PLACED — correct, since a crash between paste and submit must not
   double-deliver. But the record was then read as "this was delivered", so a send that
   FAILED verification answered `refused-duplicate` to the obvious next act. Measured
   against a Codex whose receipt channel was dead (#703's era), that was every retry;
   `--force` was the only way through.

3. **A slow render destroyed a good paste.** The `pasteFragment` verdict authorizes C-u.
   The scrape cannot distinguish "the agent rendered less than we pasted" from "the agent
   has not finished rendering what we pasted", and on an IDLE Codex it is reliably the
   latter: the pane is still (so the busy escape doesn't fire) while the draft renders a
   beat late. #697 fixed the WORKING half by treating a churning pane's placed draft as
   busy; the quiet half kept clearing a perfectly good paste and the message was lost.

## What changed

1. **A draft guard on every delivery path** (`draftBlocked`), verified and unverified alike,
   returning `state:"refused-draft"` with the draft quoted back and NOTHING written to the
   pane. Our own text in the box is not a clobber (an idempotent re-send proceeds); a pane
   with no locatable input region is not one either (nothing to protect).
   Its override is `Opts.ClobberDraft`, **separate from** the interlock's `Force` — serve
   sets Force on every `/api/send`, and merging them would have left the phone unprotected.
2. **A `failed` delivery drops its interlock record** (`ForgetSend`), so the retry is
   allowed. `queued` keeps its record: the agent accepted it, and re-sending duplicates.
3. **A placed draft is never destroyed on a hook-equipped pane.** A fragment verdict where
   something DID reach the box submits best-effort and lets the receipt judge on the FULL
   payload — a genuinely truncated paste is reported failed instead of silently mangled. A
   hook-LESS agent has no receipt, so its clear-and-retry is unchanged; that is the boundary.

## Impact

- Specs: `agent-dispatch` — one ADDED requirement (the draft guard) and two MODIFIED (the
  interlock's record lifecycle; the placed-fragment rule extended to a quiet pane).
- Code: `internal/dispatch/deliver.go`, `resend.go`, `internal/dispatchbridge`,
  `internal/app/{send,serve}.go`; tests alongside.
- Contract: `PasteAndSubmit` gains a refusal reason in its return (it returned a bare bool);
  `Result.State` gains `refused-draft`. Both are additive for JSON consumers.
- Risk: the draft guard adds ONE capture before each delivery. A false positive refuses a
  send that would have been fine — visible and recoverable. The failure it prevents is
  neither.
