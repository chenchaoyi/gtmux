# supervisor-agent (delta)

## MODIFIED Requirements

### Requirement: Lifecycle watchdog escalates a pane stuck waiting

The system SHALL, from the single-writer serve tick, escalate to a live HQ a pane that
has been WAITING (needs the user) past a timeout without being resolved — a
suggest-only nudge, fired at most ONCE per waiting episode (a marker dedups within the
episode and is cleared when the pane leaves waiting, so a fresh wait re-arms), and never
about the HQ pane itself. This complements the reclaim suggestion for a finished
dispatch (see "Reclaim suggestion when a dispatch looks done"); the watchdog only
surfaces — it never auto-reclaims or auto-answers.

The escalation SHALL re-check its premise at fire time (the standing-knock rule): a
waiting episode whose `ask` is EMPTY at escalation time SHALL NOT escalate — no question
is waiting on anyone, so nobody is stuck (measured: a codex pane held only a dim
composer placeholder with `ask: None` and had long since answered, yet `stuck·waiting`
knocked twice; the false `waiting` classification itself is the render-latency family,
tracked separately). Dim-only (`^[[2m`) composer text SHALL be treated as a placeholder,
not a draft, for this check. Skipping an ask-less escalation SHALL NOT burn the
episode's once-per-episode marker: if the same episode later gains a real ask, it may
still escalate once.

#### Scenario: A long-unresolved wait escalates

- **WHEN** a pane has been waiting past the timeout without being resolved and an HQ
  pane is live
- **THEN** HQ receives one escalation nudge for that pane, deduped per waiting episode

#### Scenario: Leaving waiting re-arms the escalation

- **WHEN** the pane leaves waiting and later enters a new waiting episode
- **THEN** a fresh escalation may fire (the prior episode's dedup does not suppress it)

#### Scenario: An ask-less wait does not escalate

- **WHEN** a pane has been waiting past the timeout but its `ask` is empty at escalation
  time (e.g. only a dim composer placeholder is visible)
- **THEN** no `stuck·waiting` escalation fires, and the episode may still escalate once
  later if a real ask appears
