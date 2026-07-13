# supervisor-agent Specification

## ADDED Requirements

### Requirement: Seeded playbook carries the full HQ charter

The generated HQ playbook SHALL encode the supervisor charter as agent-neutral,
single-source seed policy so a fresh `gtmux hq` teaches it on any workstation — not
just a machine whose operator hand-tuned it. It SHALL state: the role boundary
(orchestrate — SENSE·DECIDE·DISPATCH·SUPERVISE·REPORT — never hand-run engineering or
investigation commands, but reclamation IS HQ's responsibility, executed via
`gtmux reap` or a dispatched subagent, never hand-typed git/tmux); main-session
responsiveness (heavy/slow work goes to a subagent or separate window, never blocking
the human-input loop); dispatch granularity (one self-reporting subagent per
independent step; a fast op — reclaim/cleanup — is dispatched separately and confirmed
immediately, never chained behind a slow step); low-noise triage; human-in-the-loop for
every decision; and knowledge curation. Machine-specific facts (accounts, paths,
network, concrete footgun instances) SHALL stay in the local `knowledge/`, not the seed.

#### Scenario: A fresh home seeds the charter

- **WHEN** `gtmux hq` seeds a home
- **THEN** the playbook states the role boundary, main-session responsiveness + dispatch
  granularity, low-noise, human-in-loop, and curation — as portable policy

#### Scenario: A slow step is not chained ahead of a fast one

- **WHEN** the playbook covers dispatching a fast op (reclaim) and a slow op (release)
- **THEN** it directs dispatching them as SEPARATE self-reporting subagents so the fast
  op's completion is visible without waiting on the slow one

### Requirement: Lifecycle watchdog escalates a pane stuck waiting

The system SHALL, from the single-writer serve tick, escalate to a live HQ a pane that
has been WAITING (needs the user) past a timeout without being resolved — a
suggest-only nudge, fired at most ONCE per waiting episode (a marker dedups within the
episode and is cleared when the pane leaves waiting, so a fresh wait re-arms), and never
about the HQ pane itself. This complements the reclaim suggestion for a finished
dispatch (see "Reclaim suggestion when a dispatch looks done"); the watchdog only
surfaces — it never auto-reclaims or auto-answers.

#### Scenario: A long-unresolved wait escalates

- **WHEN** a pane has been waiting past the timeout without being resolved and an HQ
  pane is live
- **THEN** HQ receives one escalation nudge for that pane, deduped per waiting episode

#### Scenario: Leaving waiting re-arms the escalation

- **WHEN** the pane leaves waiting and later enters a new waiting episode
- **THEN** a fresh escalation may fire (the prior episode's dedup does not suppress it)
