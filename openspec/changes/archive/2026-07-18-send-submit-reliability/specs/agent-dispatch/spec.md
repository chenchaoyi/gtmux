# agent-dispatch (delta)

## MODIFIED Requirements

### Requirement: Delivery via paste buffer, not literal send-keys

The system SHALL deliver task text using a tmux paste buffer (`load-buffer` then
`paste-buffer`), NOT `send-keys -l`. The paste SHALL be BRACKETED (`paste-buffer
-p`), so that an agent TUI receives a multi-line payload as one insertion into its
input box: sent raw, every newline reaches the TUI as a bare Return and submits the
line then and there, splitting one instruction into several messages. Delivery and
submission (Enter) SHALL be separate steps so verification can run between them and
re-submit independently. Because `paste-buffer -p` brackets only when the
application has bracketed-paste mode enabled at that instant, the system SHALL NOT
rely on bracketing alone for atomicity: it SHALL confirm the input draft holds the
delivery before sending Enter (see "Landing is the only success"), so a paste that
streamed raw newlines (submitting a line early) or left an unterminated paste state
(which would make a later Enter insert a newline instead of submitting) is detected
as a draft that does not hold the full text, and is retried or reported — never
submitted as a fragment. This applies to EVERY text-into-a-pane path — the verified
dispatch, `gtmux send` with verification skipped, and `POST /api/send` — which differ
only in whether they confirm the LANDING after submit, not in whether they confirm
the DRAFT before it.

#### Scenario: Task text is pasted, then submitted separately

- **WHEN** a task is delivered to a pane
- **THEN** the text is loaded into a tmux buffer and pasted into the pane, and
  Enter is sent as a distinct, separately-verifiable step

#### Scenario: A multi-line instruction is one message, not one per line

- **WHEN** a delivery whose text contains newlines is pasted into an agent pane
- **THEN** the whole text lands in the input draft as a single unsubmitted block, and
  the separate Enter submits it exactly once

#### Scenario: A paste that did not bracket is not submitted as a fragment

- **WHEN** a multi-line paste arrives while the agent's bracketed-paste mode is off,
  so newlines submit early and the draft no longer holds the full payload
- **THEN** the draft fails the full-content check and Enter is not sent against the
  partial draft — the delivery is retried or reported, never submitted as-is

### Requirement: Landing is the only success; fragments and swallowed Enter are handled

Delivery SHALL be considered successful ONLY when the delivery is confirmed landed
(by the stream, or by the hardened fallback). Before submitting, the system SHALL
confirm the FULL task text is present in the input draft — both the leading
fingerprint (head) AND the trailing fingerprint (tail) of the payload, or a TUI's
collapsed-paste placeholder that stands in for a folded large paste; a match on the
head alone (a prefix) SHALL NOT authorize submission, because a half-rendered draft
whose tail has not yet arrived would otherwise be submitted truncated and then
misread as landed. A partial/fragment paste SHALL be retried or reported as failed,
never submitted as-is. A submission whose Enter was swallowed (the text remains in
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

## ADDED Requirements

### Requirement: Unverified send paths confirm the draft before submitting

The system SHALL confirm that the input draft holds the FULL delivered text (head
AND tail, or a collapsed-paste placeholder) before sending Enter on the unverified
text paths — `POST /api/send` (the phone / menu-bar reply) and `gtmux send` with
verification skipped — using the same draft-content check as the verified dispatch. These
paths SHALL still skip the post-submit LANDED verification (to stay within the
phone's latency budget), so they differ from verified dispatch only in whether they
confirm the landing AFTER submit — not in whether they race paste against Enter. The
pre-submit confirmation SHALL be bounded by the same settle window (a healthy paste
confirms within a frame, so the fast path stays fast); if the window elapses without
a full-draft match, the path MAY still send Enter best-effort but SHALL NOT be
required to report success.

#### Scenario: The phone reply does not race paste against Enter

- **WHEN** a multi-line reply is sent via `POST /api/send` with `enter:true`
- **THEN** the text is pasted and the Enter is withheld until the draft is confirmed
  to hold the full text (or the settle window elapses), so the reply submits as one
  whole message rather than a truncated fragment

#### Scenario: A short single-line send stays fast

- **WHEN** a short line is sent via `POST /api/send` and renders in the draft within
  one frame
- **THEN** the confirmation passes immediately and Enter follows without added delay
