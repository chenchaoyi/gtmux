# chat-transcript (delta)

## ADDED Requirements

### Requirement: A turn says who delivered its prompt

A served turn SHALL carry the SENDER of its prompt whenever gtmux itself delivered that
prompt on someone else's behalf, and SHALL carry nothing when it did not.

The reader's own channels are not senders. Typing in the pane, sending from the phone, and
sending from the browser are all the person reading the conversation, and a turn from any
of them SHALL be unmarked, which is how every turn renders today.

Attribution SHALL come from gtmux's own record of the delivery, never from the prompt's
text: the audit journal records each delivery's target pane, a bounded head of its payload,
and the sender in the action log's actor vocabulary. A turn is matched to a delivery by
pane and by a normalized head of the text, case and whitespace folded.

An unmatched turn SHALL be unmarked rather than guessed. The journal is rotated by size, so
a turn older than the journal cannot be attributed, and saying nothing is the only honest
answer available.

#### Scenario: The supervisor relays an instruction

- **WHEN** HQ sends a message into a worker's pane
- **THEN** that turn is served marked as HQ's, and the chat draws HQ's own mark rather than
  the reader's avatar

#### Scenario: The reader sends from their phone

- **WHEN** the person reading sends a message from the phone or the browser
- **THEN** the turn is unmarked, because it was them

#### Scenario: Another agent dispatches a task

- **WHEN** one agent delivers a task into another agent's pane
- **THEN** that turn is marked with the sending session, so a dispatch is not read as the
  reader's own instruction

#### Scenario: Nothing to join against

- **WHEN** a turn is older than the audit journal, or no delivery matches it
- **THEN** it is served unmarked, and renders exactly as it did before this existed
