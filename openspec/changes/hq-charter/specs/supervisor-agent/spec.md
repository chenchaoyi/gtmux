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

### Requirement: Lifecycle watchdog surfaces reclaimable and stuck sessions

The system SHALL, from the single-writer serve tick, surface two lifecycle conditions
to a live HQ as deduped, snoozeable, suggest-only nudges: a finished dispatch OR a
lingering window whose worktree is merged and clean (a `reap-suggest`, covering
manually-created windows via reap-by-pane), and a pane that is stuck (working with no
output past a threshold, or waiting/errored past a timeout) (an escalation). It SHALL
never auto-reclaim and never auto-answer — it only surfaces.

#### Scenario: A lingering merged window is suggested for reclaim

- **WHEN** a window's worktree is merged and clean and its work is done
- **THEN** HQ receives one `reap-suggest` (deduped), and nothing is reclaimed automatically

#### Scenario: A stuck pane escalates

- **WHEN** a pane is working with no output past the threshold, or waiting/errored past
  the timeout
- **THEN** HQ receives one escalation nudge, deduped and snoozeable
