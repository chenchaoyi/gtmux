## MODIFIED Requirements

### Requirement: Deterministic per-agent cognitive digest

The system SHALL assemble, on demand and without any LLM call, a digest for every
radar row (tmux and native) joining: identity (pane/loc/agent/source,
project/branch), state (waiting/working/idle/running + waiting kind + since +
errored/background markers), goal (the session's last user prompt), last (the
tail of the last assistant reply), when waiting — ask (the parsed prompt
options text), and the session's USAGE snapshot (`tok`, `ctx` 0–1, `rate`,
`usage_warn` — see `usage-watch`). Fields whose source is absent SHALL degrade
to empty without failing the row (zero-intrusion: agents need not cooperate).
The CLI SHALL remain cgo-free.

#### Scenario: Digest of a waiting agent

- **WHEN** an agent pane is waiting on a permission/plan/question
- **THEN** its digest row carries state=waiting with the kind, the goal from its
  transcript, and the ask text parsed from the live pane

#### Scenario: Sparse session degrades gracefully

- **WHEN** a session has no on-disk transcript (e.g. a just-started agent)
- **THEN** the digest row still renders from radar signals alone, with
  goal/last/ask and the usage fields empty

#### Scenario: Digest carries usage

- **WHEN** a session has usage data and a breached/projected layer
- **THEN** its digest row includes tok/ctx/rate and the `usage_warn` string
