# send-feedback-on-the-phone — what the app tells you about a message it just sent

**Status: a candidate for the commander. Nothing here is implemented.** It was written
overnight on 2026-09-04 out of an exploration round, and it exists because two decisions
are display decisions, which are the commander's, not the explorer's.

## Why

Two things the phone cannot currently say about a send, both discovered by driving the
paths rather than reading them.

**1. A refused send does not say why.** The core refuses with reasons that are specific
and actionable, and the client dropped every one of them at the boundary, so all three
arrived as one undifferentiated failure bar:

| the core's refusal | what it means to the reader |
|---|---|
| `refused-draft` | someone is typing in that pane right now; sending would submit their half-written line along with yours |
| `no such pane` | the session is gone |
| `key not allowed` | the app asked for a control key the server will never run (the reader cannot cause this) |

The client now KEEPS the reason (`sendResult`, shipped — additive, nothing on screen
moved). What the reader is told is undecided.

**2. A send into a WORKING session cannot be told from one into an idle session.** The
phone shows "sent" for both. The message is not lost: the agent queues it behind its
current turn. But "delivered" and "queued behind a turn that may run for minutes" are
different facts, and the reader acts on them differently.

A correction that matters for anyone reasoning about this: `/api/send` goes through
`dispatch.PasteAndSubmit`, **not** `Deliver`. The queued detection (`queuedMarkers`, read
off the screen) lives in Deliver's post-submit verification loop, which the phone's path
never enters — so the server answers a plain `{"status":"ok"}` either way. **The queue is
the agent's; the server does not know about it, and no field carries it across the API.**
This is not a display bug. The information does not arrive.

## What to decide

### Decision 1 — what a failed send says

Suggested: one failure bar carrying the core's own sentence, no dialog. Only
`refused-draft` earns a second action ("send anyway"), because it is the only refusal the
reader can override — and it must be explicit, since overriding it submits someone else's
unfinished line. `key not allowed` should stay in the log; a reader cannot cause it.

### Decision 2 — telling a queued send from a delivered one

**Form A — measure it.** After `PasteAndSubmit`, serve reads the screen once and sets a
`queued` bit on the `/api/send` response when it matches `queuedMarkers`. The phone then
says "queued — it will be handled when this turn ends".
*Cost:* one screen read on a path built to return fast (the input echo is what makes the
phone feel immediate). *Benefit:* it is measured, not inferred.

**Form B — infer it, no API change.** The phone already knows the target's status from the
radar. If it is `working` at send time, say so in the client: "it is running; this will be
handled when the turn ends".
*Cost:* an inference, not a measurement — the status can be briefly wrong (a hook that
missed a transition). *Benefit:* no server change, no added latency, and it matches the HQ
charter's existing rule that sending into a working session deserves a warning first.

**HQ's preference: B first, A only if a precise receipt is ever actually needed.** Mine
is the same, for the reason that B can be decided in a morning and A cannot.

## Background the display decision will want

Retries are told from new messages by the idempotency token, not by the text: the app
reuses `send_id` for a Retry (so an ambiguous timeout cannot double-send) and mints a new
one for a new message. Whatever the failure bar offers, "Retry" must keep the token and
"send another" must not.

## Impact

None until a decision is taken. Decision 1 touches the mobile app only; Decision 2 form A
touches `api/contract.md`, `internal/server`, `internal/app/serve.go` and the app, while
form B touches the app only.
