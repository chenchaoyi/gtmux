# agent-dispatch (delta)

## ADDED Requirements

### Requirement: Supervisor-driven sends and reaps are journaled

A `gtmux send` TEXT delivery SHALL, at settlement, append a `gtmux:audit:send`
record naming the target pane, the outcome state, and a bounded head of the
payload — on every text path (verified, unverified, and plain-terminal). A
`--key` send SHALL append nothing (a single whitelisted keystroke carries no
payload worth a record). This gives the sender's side of the story a durable
home: the target pane's own hook event carries the prompt's head but cannot say
WHO drove it, and the re-send interlock keeps only an overwritten hash.

A reap that actually reclaims a dispatch SHALL append a `gtmux:audit:reap`
record naming the task id, the pane, and the actions taken — written BEFORE the
ledger entry is removed, so the journal keeps what the ledger forgets. A snooze
SHALL append nothing: it is already persisted on the task it silences.

Both are audit records under the session-events audit rules: trail, not debt.

#### Scenario: A send is attributable after the fact

- **WHEN** `gtmux send %28 "check the failing test"` settles as landed
- **THEN** a `gtmux:audit:send` record names `%28`, the landed state, and the
  payload head — so "what did the supervisor send that pane" is a journal query

#### Scenario: A refused send is journaled too

- **WHEN** a send is refused (a drafted box, a duplicate payload)
- **THEN** the audit record carries the refused state, so the journal shows the
  attempt as well as the outcome

#### Scenario: A reap outlives its ledger entry

- **WHEN** `gtmux reap <id>` reclaims a dispatch and removes its task file
- **THEN** a `gtmux:audit:reap` record with the task id and actions remains in the
  journal

#### Scenario: A keystroke is not a payload

- **WHEN** `gtmux send %28 --key Escape` delivers
- **THEN** no audit record is written
