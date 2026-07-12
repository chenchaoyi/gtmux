# Agent Digest Specification

## ADDED Requirements

### Requirement: Deterministic per-agent cognitive digest

The system SHALL assemble, on demand and without any LLM call, a digest for every
radar row (tmux and native) joining: identity (pane/loc/agent/source,
project/branch), state (waiting/working/idle/running + waiting kind + since +
errored/background markers), goal (the session's last user prompt), last (the
tail of the last assistant reply), and — when waiting — ask (the parsed prompt
options text). Fields whose source is absent SHALL degrade to empty without
failing the row (zero-intrusion: agents need not cooperate). The CLI SHALL remain
cgo-free.

#### Scenario: Digest of a waiting agent

- **WHEN** an agent pane is waiting on a permission/plan/question
- **THEN** its digest row carries state=waiting with the kind, the goal from its
  transcript, and the ask text parsed from the live pane

#### Scenario: Sparse session degrades gracefully

- **WHEN** a session has no on-disk transcript (e.g. a just-started agent)
- **THEN** the digest row still renders from radar signals alone, with
  goal/last/ask empty

### Requirement: Digest CLI

The system SHALL provide `gtmux digest` printing one compact human-readable
block per agent (bilingual labels per `GTMUX_LANG`), and `gtmux digest --json`
emitting a machine-readable array — the supervisor's primary read surface.

#### Scenario: Fleet at a glance

- **WHEN** the user (or the supervisor agent) runs `gtmux digest --json`
- **THEN** every radar row appears with the digest fields, ordered like the
  radar (needs-you first)

### Requirement: Digest over the API

The system SHALL expose `GET /api/digest` (bearer-gated like every `/api/*`)
returning the same JSON array, additively — existing API contracts unchanged.

#### Scenario: Remote digest read

- **WHEN** an authenticated client GETs `/api/digest`
- **THEN** it receives the digest array; without a token the request is rejected
  like any other `/api/*`
