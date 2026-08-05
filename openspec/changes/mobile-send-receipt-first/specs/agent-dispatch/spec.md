# agent-dispatch (delta)

## ADDED Requirements

### Requirement: The mobile send path confirms by the agent's receipt, with the screen-scrape only a pre-check

A message delivered via `POST /api/send` to a hook-equipped agent pane SHALL treat the
agent's `UserPromptSubmit` receipt — matched on the prompt's normalized head — as the
authority for whether the message landed. The pre-submit screen-scrape (the input-box
draft read) SHALL be used only to sanity-check delivery before submitting (e.g. drop the
pane out of copy-mode, observe that the paste reached the box); it SHALL NOT be the sole
arbiter of "did it send", because it is coupled to each agent's TUI and drifts across
agents and versions.

Delivery SHALL remain optimistic and fast: the send SHALL paste, submit best-effort, and
return with the input echo without blocking on the receipt. Confirmation SHALL be
reconciled asynchronously against the event stream.

#### Scenario: A confirmed draft still returns fast

- **WHEN** the pre-submit guard reads the full delivery back in the input box
- **THEN** the send submits and returns "sent" immediately, without waiting for the receipt

#### Scenario: Confirmation comes from the receipt when the scrape cannot read the box

- **WHEN** the pre-submit scrape cannot read the delivery back (a working pane whose
  composer the scrape cannot cleanly capture) but the agent later fires a
  `UserPromptSubmit` whose head matches the delivery
- **THEN** the send is confirmed by that receipt, not by the scrape

#### Scenario: No receipt within the grace window surfaces uncertainty, not a scrape verdict

- **WHEN** an optimistically submitted send produces no matching receipt within the grace
  window
- **THEN** the app surfaces "might not have sent" rather than reporting failure from a
  single brittle scrape frame

### Requirement: A working pane never hard-fails a synchronous send

When the target pane is busy — the radar reports it working, or whole-frame motion shows
it still rendering — `POST /api/send` SHALL NOT synchronously reject the send as a
fragment. A paste that demonstrably reached the box SHALL be submitted best-effort. A
message that still cannot be confirmed SHALL be HELD and re-delivered when the pane next
goes idle (the radar working→idle transition), rather than dropped with an error.

Re-delivery SHALL be idempotent: a held message carries a stable id so a retry, a
reconnect, or the idle-trigger firing more than once never delivers it twice.

#### Scenario: Busy now, delivered when idle

- **WHEN** a send cannot be confirmed against a working pane and is held
- **THEN** it is delivered exactly once when the pane transitions to idle

#### Scenario: A non-agent pane stays best-effort

- **WHEN** the target is a plain shell pane (no hook receipt, no structured input box)
- **THEN** the bytes are delivered fire-and-forget best-effort, with no false claim of an
  app-level confirmation
