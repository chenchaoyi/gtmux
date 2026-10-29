## MODIFIED Requirements

### Requirement: A verified Codex terminal client keeps its managed identity

The core SHALL admit process-backed Codex pane ownership only after verifying a stable foreground client, its actual terminal stdin, fresh pane incarnation, exact process birth, same home and matching explicit CLI rollout identity. Explicit `originator=codex-tui` creation MAY use the known string sources `cli` or `vscode`; neither source alone SHALL prove current terminal ownership. Unknown, missing or subagent sources and non-TUI origins SHALL remain ineligible for this recovery path. Titles, directories, inherited pane values, creation metadata or a shared backend PID alone SHALL NOT authorize a binding. Existing delivery nonce validation SHALL retain priority.

A current main-session start or submit hook MAY use its own verified client identity. Reconciliation of stored observations SHALL require exactly one eligible session for that client and unchanged observations/incumbent state before commit. A newly admitted identity SHALL parse as a real conversation; empty startups SHALL defer and known helpers or command-only stubs SHALL NOT claim the client. It SHALL preserve native history, reject changed or ambiguous evidence and suppress only the exact bound session's duplicate external row.

#### Scenario: HQ starts a new conversation in a reused pane

- **WHEN** the pane still names an older conversation but the new CLI session has an exact verified foreground client
- **THEN** chat, resume and list consumers use the new binding and emit only one managed pane row
- **AND** existing send guards and external access policies remain

#### Scenario: A TUI rollout carries an editor backend source

- **WHEN** explicit TUI creation carries `source=vscode` and its exact foreground terminal client is independently verified
- **THEN** recovery accepts that source as it accepts `source=cli`
- **AND** editor origin or a shared backend PID cannot replace the foreground evidence

#### Scenario: A shared backend or changed process is observed

- **WHEN** stdin is piped, the client is not foreground, its birth/pane changes, the home or rollout is mismatched, or multiple sessions claim it
- **THEN** recovery does not guess a binding or remove native history
