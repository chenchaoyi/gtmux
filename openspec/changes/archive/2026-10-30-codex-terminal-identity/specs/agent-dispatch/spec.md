## MODIFIED Requirements

### Requirement: Delivery via paste buffer, not literal send-keys

For terminal delivery, the system SHALL deliver task text using a tmux paste buffer (`load-buffer` then
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

A live kernel-verified Codex frontend MAY instead use addressed `thread/queue/add` after the same draft, waiting, lease and target-recheck guards. It SHALL verify home and backend incarnation, bind a stable message ID and require a matching queue receipt. The durable exclusive attempt claim SHALL precede mutation; uncertain attempts SHALL NOT fall back to paste/Enter or replay. Queue acceptance SHALL NOT be reported as turn execution. Existing authorized upload images SHALL retain image input semantics; other files remain text paths.

#### Scenario: Shared Codex accepts input during a turn
- **WHEN** its exact current home/thread accepts the guarded input with a matching queue receipt
- **THEN** the result is queued rather than executed, and no terminal paste or Enter occurs
- **AND** repeating a settled delivery ID does not create a second input

#### Scenario: Native acceptance is uncertain
- **WHEN** the connection fails after the input may have been written
- **THEN** a durable uncertain attempt remains and no automatic replay or terminal fallback occurs
