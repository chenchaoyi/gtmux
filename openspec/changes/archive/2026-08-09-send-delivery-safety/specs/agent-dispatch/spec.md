# agent-dispatch (delta)

## ADDED Requirements

### Requirement: Delivery never writes into an unsubmitted draft

A paste APPENDS to a pane's input box; it does not replace it. So before delivering, the
system SHALL read the target's input draft and SHALL REFUSE the delivery
(`state:"refused-draft"`, writing nothing to the pane) when it holds UNSUBMITTED text that
is not the delivery itself — otherwise the payload is concatenated onto someone's
half-written line and both are submitted as one message, without its author ever pressing
Enter. The refusal SHALL quote enough of the draft for the caller to recognize whose text
it protected.

Two cases SHALL NOT be treated as a clobber: a draft that already holds THIS delivery (an
idempotent re-send after a lost acknowledgement), and a pane with no locatable input region
(a plain shell — there is no draft to protect).

The draft SHALL be read from the COLOR capture, so text the agent renders FAINT — its
suggested-next-command ghost, which is not input — is excluded; a plain capture cannot
distinguish the two and refuses sends to panes whose box is in fact empty. The refusal
SHALL require TWO agreeing reads: one frame caught mid-repaint can show a phantom, and a
refusal cannot be taken back the way the wake channel's queue-and-retry can.

The guard SHALL apply to every delivery path, verified and unverified alike, INCLUDING the
phone's `POST /api/send` — a send from another device into a pane whose owner is typing is
the case with no one present to undo it. Its override SHALL be a DISTINCT option from the
re-send interlock's `--force`: the phone sets the interlock override on every send (it
carries its own idempotency key), and folding the two together would leave that surface
unprotected. The operator's `gtmux send --force` SHALL waive both.

#### Scenario: A user's half-written line is not swallowed

- **WHEN** a delivery targets a pane whose input box holds text the user has not submitted
- **THEN** nothing is pasted, no Enter is sent, and the delivery is refused as
  `refused-draft` with the draft quoted back

#### Scenario: The agent's own ghost text is not a draft

- **WHEN** a pane's input box is empty but the agent renders a FAINT suggested-next-command
  where a draft would be
- **THEN** the delivery proceeds — the suggestion is not input

#### Scenario: Our own re-send is not a clobber

- **WHEN** the draft already holds the delivery being sent (a retry after a lost ack)
- **THEN** the delivery proceeds and confirms idempotently

#### Scenario: The phone cannot waive draft protection

- **WHEN** `POST /api/send` delivers to a pane holding an unsubmitted draft
- **THEN** it is refused and the client is told the pane has unsent text — the interlock
  override the phone always carries does not waive this guard

#### Scenario: The operator can override deliberately

- **WHEN** `gtmux send --force` targets a pane with an unsubmitted draft
- **THEN** the delivery proceeds

## MODIFIED Requirements

### Requirement: Re-send interlock refuses an identical duplicate

Before delivering, the system SHALL record a hash of the payload per pane. An
IDENTICAL payload delivered to the same pane within a configurable `resendWindow`
SHALL be REFUSED (delivering nothing, `state:"refused-duplicate"`) unless an explicit
`--force` is given. This seals off a nervous duplicate of a side-effecting command
(e.g. a second `/compact`) while leaving a deliberate repeat available via `--force`.
The interlock SHALL NOT block a different payload, nor a repeat after the window lapses.

The record SHALL be written when the paste is PLACED (so a failure between paste and
submit cannot double-deliver) and SHALL be DROPPED when the delivery ends `failed`: a
delivery that never landed must not refuse its own retry, which is the obvious next act
and was answerable only with `--force`. A delivery reported `queued` was ACCEPTED and
SHALL keep its record — re-sending it would duplicate the instruction the interlock exists
to protect.

#### Scenario: A duplicate within the window is refused

- **WHEN** the same payload is delivered to the same pane twice within `resendWindow`
  and `--force` is not given
- **THEN** the second delivery is refused (`state:"refused-duplicate"`) and nothing is
  sent

#### Scenario: Force overrides the interlock

- **WHEN** the same payload is delivered again with `--force`
- **THEN** the delivery proceeds

#### Scenario: A different payload is unaffected

- **WHEN** a different payload is delivered to the same pane within the window
- **THEN** it is delivered normally

#### Scenario: A failed delivery may be retried immediately

- **WHEN** a delivery ends `failed` and the same payload is sent again within the window
- **THEN** it is delivered rather than refused — the failed attempt's record was dropped

#### Scenario: A queued delivery still holds the interlock

- **WHEN** a delivery is reported `queued` and the same payload is sent again within the
  window
- **THEN** it is refused, because the agent already accepted the instruction

### Requirement: Landing is the only success; fragments and swallowed Enter are handled

Delivery SHALL be considered successful ONLY when the delivery is confirmed landed
(by the stream, or by the hardened fallback). Before submitting, the system SHALL
confirm the FULL task text is present in the input draft — both the leading
fingerprint (head) AND the trailing fingerprint (tail) of the payload, or a TUI's
collapsed-paste placeholder that stands in for a folded large paste; a match on the
head alone (a prefix) SHALL NOT authorize submission, because a half-rendered draft
whose tail has not yet arrived would otherwise be submitted truncated and then
misread as landed. A payload line that is a bare image path (the mobile composer's
attachment shape) SHALL additionally be accepted as placed when the draft shows an
attachment chip (e.g. Claude Code's `[Image #N]`) standing in for it — the TUI folds
a pasted image path into the chip immediately, so the path text itself never appears
in the draft; any remaining prose in the payload must still match head AND tail on
its own, and a chip SHALL NOT vouch for a payload that had no image-path lines. A partial/fragment paste SHALL be retried or reported as failed,
never submitted as-is. The fragment verdict SHALL be reserved for a QUIET pane whose
draft settled short: a paste that demonstrably reached the box (a non-empty draft) on a
pane whose transcript is STILL redrawing frame-to-frame (the target agent mid-turn, so no
clean frame ever reads back the full head+tail) SHALL NOT be treated as a fragment —
clearing or withholding there silently drops the send into a working pane and the
destructive draft-clear mangles the good paste — but SHALL be submitted best-effort,
leaving the post-submit landing check (verified paths) or the agent's own submit receipt
(fast path) as the authority.

For a HOOK-EQUIPPED agent that protection SHALL extend to a QUIET pane as well: when a
paste demonstrably reached the box, a fragment verdict SHALL NOT authorize clearing and
re-pasting, because the screen cannot distinguish "the agent rendered less than we pasted"
from "the agent has not finished rendering what we pasted", and the second case is the
common one on an idle pane whose TUI redraws a beat late. The delivery SHALL be submitted
and judged by the receipt, which matches the FULL payload — so a genuinely truncated paste
is reported failed rather than silently mangled. A hook-LESS agent has no receipt, so the
draft scrape remains its only evidence and the clear-and-retry SHALL still apply there. A submission whose Enter was swallowed (the text remains in
the draft and no submit event arrived) SHALL be resubmitted with backoff, and each
resubmit SHALL re-confirm the draft STILL holds the full text first — the system
SHALL NOT re-send Enter blindly against a draft that is empty (already submitted) or
no longer matches. If verification does not succeed within the timeout, the system
SHALL report `delivered:false` (`state:"failed"`) together with on-screen evidence
(a capture of the pane) and SHALL NOT report success.

#### Scenario: Fragment is not silently accepted

- **WHEN** only a prefix of the task text lands in the input draft (e.g. `"cl"`)
- **THEN** the paste is retried, and if it still cannot place the full text the
  result is `delivered:false` with evidence — never a claimed success

#### Scenario: A head-only draft is not submitted as the whole task

- **WHEN** the draft shows the payload's first lines (the head matches) but the tail
  has not rendered yet
- **THEN** submission waits for the tail within the settle window; a draft holding
  only the head is treated as a fragment, not submitted as the complete task

#### Scenario: A churning pane's placed paste submits best-effort, not treated as a fragment

- **WHEN** the paste has reached the input box (the draft is non-empty) but the pane's
  transcript keeps redrawing every frame — the target agent is mid-turn — so no clean
  frame ever reads back the full head+tail within the settle window
- **THEN** the delivery is judged a busy pane, not a fragment: the draft is NOT cleared
  and Enter IS sent (best-effort), and the landing is decided by the submit receipt /
  post-submit check — a send into a working `%pane` is neither silently withheld nor
  mangled by a destructive draft-clear
- **AND WHEN** instead the pane is quiet (its transcript is static) and the draft settled
  short, the fragment handling above still applies — retried or reported, never submitted

#### Scenario: An image path folded into an attachment chip still submits

- **WHEN** the payload is (or ends with) a bare image path — the phone's uploaded
  photo — and the agent's TUI immediately folds it into an `[Image #N]` chip so the
  path text never appears in the draft
- **THEN** the chip (plus a head+tail match on any remaining prose) counts as the
  delivery placed, and Enter is sent — the photo is not stranded placed-but-unsent;
  a chip alone SHALL NOT confirm a payload that contained no image-path lines

#### Scenario: Swallowed Enter is retried, but only against a matching draft

- **WHEN** the task text is pasted but the submitting Enter is swallowed (the full
  text remains in the draft and no submit event appears)
- **THEN** Enter is re-sent with backoff after re-confirming the draft still holds
  the full text; once the draft is empty or no longer matches, no further Enter is
  sent, and the timeout yields `delivered:false` + evidence if never confirmed

#### Scenario: Empty box without a submit is not "working"

- **WHEN** the input box is empty but no submission was confirmed (nothing actually
  entered the conversation)
- **THEN** the result is `delivered:false` — an empty box plus a nonzero token
  counter is NOT accepted as evidence of work

#### Scenario: Timeout never reports success

- **WHEN** verification does not confirm within the deliver timeout
- **THEN** the result is `delivered:false` with a capture of the current screen
