# supervisor-agent (delta)

## MODIFIED Requirements

### Requirement: gtmux capture records a distill candidate to the pending-distill spool

The system SHALL provide `gtmux capture "<one-line lesson> @<topic>"` — topic ∈ the
knowledge vocabulary: the built-in topics (accounts | workflows | best-practices |
pitfalls | corrections | environment) plus every topic DECLARED through
`gtmux knowledge topic` — as a PUBLIC command so that HQ OR ANY WORKER can record a
durable-fact CANDIDATE cheaply in the moment, widening the capture surface from a
single supervisor to the whole fleet. Capture and the knowledge verbs SHALL judge
topics through one shared validation, so the two entrances cannot drift. It SHALL be
safe to open this input because a candidate is NOT a KB entry: the distillation pass
(HQ's curation) is the quality gate. The command SHALL append one JSON line — the
lesson, the topic TAG, a DEDUP KEY (topic + a lesson slug, or an explicit key), and
AUTO-COLLECTED event context (current/related `pane_id`, the current event `seq`,
`task_id` if any, a timestamp) — to a pending-distill spool under the HQ home
(`~/.config/gtmux/hq/knowledge/.pending-distill.jsonl` or the state-dir equivalent).
`gtmux capture --list` SHALL render the pending queue. A missing or invalid `@topic`
SHALL be an error naming the current vocabulary. The spool SHALL drain candidate by
candidate, never by blind truncation: `gtmux knowledge add --capture <key>` consumes
EVERY pending line sharing that key (the merge of same-key candidates) into one entry
that inherits their provenance, and `gtmux knowledge dismiss --capture <key> --why`
removes them with a journal trace — so an accepted and a rejected candidate stop
vanishing identically. Being a public command it SHALL be documented per the
command-drift rule: the CLAUDE.md command list, a `docs/cli.md` section, and
`gtmux --help` (en+zh) — NOT the `check-design.sh` HIDDEN allowlist.

#### Scenario: A candidate is captured in one line with a dedup key

- **WHEN** any worker runs `gtmux capture "wrangler TLS-resets from the office; retry @pitfalls"`
- **THEN** one JSON line — lesson + topic tag `pitfalls` + a dedup key + auto-collected
  pane/seq/task/time context — is appended to the pending-distill spool

#### Scenario: A declared custom topic is capturable

- **WHEN** HQ has declared a `datasets` topic and a worker runs
  `gtmux capture "… @datasets"`
- **THEN** the candidate is accepted and tagged `datasets`, exactly as a built-in
  topic would be

#### Scenario: An invalid topic errors

- **WHEN** `gtmux capture` is called with no `@topic` or a topic outside the
  current vocabulary
- **THEN** the command errors naming the vocabulary and writes nothing

#### Scenario: Distill merges same-key candidates rather than duplicating

- **WHEN** two candidates share a (topic, dedup key) and a distill pass drains the spool
- **THEN** one `gtmux knowledge add --capture <key>` consumes both into ONE entry (no
  near-duplicate), the entry's provenance carries both sequences, and the remaining
  unrelated candidates stay pending
