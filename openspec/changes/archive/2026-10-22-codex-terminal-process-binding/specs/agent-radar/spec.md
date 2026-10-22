## ADDED Requirements

### Requirement: A verified Codex terminal client keeps its managed identity

The core SHALL admit process-backed Codex pane ownership only after verifying a stable foreground client, its actual terminal stdin, fresh pane incarnation, exact process birth, same home and matching explicit CLI rollout identity. Titles, directories, inherited pane values, creation metadata or a shared backend PID alone SHALL NOT authorize a binding. Existing delivery nonce validation SHALL retain priority.

A current main-session start or submit hook MAY use its own verified client identity. Reconciliation of stored observations SHALL require exactly one eligible session for that client and unchanged observations/incumbent state before commit. It SHALL preserve the existing rule that command-only stubs cannot displace a real conversation. It SHALL preserve native history, reject changed or ambiguous evidence and suppress only the exact bound session's duplicate external row.

#### Scenario: HQ starts a new conversation in a reused pane

- **WHEN** the pane still names an older conversation but the new CLI session has an exact verified foreground client
- **THEN** chat, resume and list consumers use the new binding and emit only one managed pane row
- **AND** existing send guards and external access policies remain

#### Scenario: A shared backend or changed process is observed

- **WHEN** stdin is piped, the client is not foreground, its birth/pane changes, the home or rollout is mismatched, or multiple sessions claim it
- **THEN** recovery does not guess a binding or remove native history
