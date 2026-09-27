# chat-transcript Specification Delta

## ADDED Requirements

### Requirement: Every served turn carries its source agent identity

Every transcript turn SHALL carry the canonical agent key of the session that produced it.
The current session identity SHALL come from its resume binding. A prior HQ session SHALL use
the agent key persisted in its replacement record; legacy links may use unique log-based
inference. If identity remains ambiguous, the server SHALL not label that turn as another
agent.

#### Scenario: HQ switches from Claude to Codex

- **WHEN** the phone loads a history chain containing a Claude predecessor and a Codex current
  session
- **THEN** each turn carries its own source agent and the reader can distinguish the boundary

### Requirement: Codex user prompts support current and legacy rollout records

The Codex transcript parser SHALL read user input from both legacy `event_msg.user_message`
records and observed `response_item` message records with `role: user` and `input_text` blocks.
It SHALL filter injected repository instructions and environment context, collapse duplicate
representations of the same adjacent prompt, and continue past unknown records.

#### Scenario: Current Codex rollout records a user prompt

- **WHEN** a Codex rollout contains a user `response_item` followed by recognized reply events
- **THEN** the chat shows that prompt paired with its reply and omits injected setup text
