# hq-attention-system (delta)

## MODIFIED Requirements

### Requirement: Degradation is surfaced as CRITICAL

The system SHALL surface any perception-layer degradation to the user as a CRITICAL
condition immediately — the feed down after failed self-heal, a stale stream, or a
detected cursor gap — via a synthetic `feed-degraded` control record marked `important`
appended to the session JOURNAL (from which the feed daemon's own tail spools it — one
record, not two; a record written only to the spool reaches no reader, the measured #647
failure shape) AND one visible HQ-pane nudge, so a perception outage is known within
seconds rather than discovered long after. Recovery SHALL clear the degradation state
without re-alerting on the recovery.

#### Scenario: An outage is announced at once

- **WHEN** the feed is judged down (failed self-heal) or the stream is stale
- **THEN** a CRITICAL degradation is surfaced to the user immediately, stating the feed
  is down and a polling backstop is in effect — and the control record is visible to
  `gtmux events` and counts toward the consumption debt

#### Scenario: Recovery does not re-alert

- **WHEN** a previously degraded feed becomes healthy again
- **THEN** the degradation state clears and no new alert fires for the recovery itself

### Requirement: Startup reconciliation

On every (re)start of the perception feed, the system SHALL rebuild state from two sources
— replay the journal from the cursor AND pull one full `digest` snapshot — so a single
restart never loses state. The reconciliation SHALL be idempotent (safe to run on a
spurious gap). The `reconcile` control record announcing it SHALL be appended to the
session journal (not only the spool), so feed restarts are visible to the pull side and
auditable from the stream.

#### Scenario: A restart rebuilds without loss

- **WHEN** the feed (re)starts
- **THEN** it replays outstanding journal events from the cursor and takes one full
  digest snapshot, reconstructing the current fleet state — and a `gtmux:reconcile`
  control record lands in the journal
