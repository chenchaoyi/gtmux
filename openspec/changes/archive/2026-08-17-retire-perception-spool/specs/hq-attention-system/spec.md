# hq-attention-system (delta)

## REMOVED Requirements

### Requirement: Perception feed daemon

**Reason**: the spool's only reader (`hq-feed --tail`) is off the playbook's
recommended path; the daemon copied the journal for a subscriber its own
charter says does not exist, and the copy is the measured #647 loss class
(two streams, two truths).

### Requirement: Heartbeat and mechanical watchdog

**Reason**: the watchdog supervised the copyist. The zero-loss guarantees
(journal seq + watermark ack + unread re-knock) never depended on the daemon
and keep working with it gone.

### Requirement: Startup reconciliation

**Reason**: the reconcile record announced DAEMON restarts. HQ's own restarts
rebuild via the startup briefing's digest snapshot; gap handling moves to the
read path (see the modified catch-up requirement).

## MODIFIED Requirements

### Requirement: Split feeding-HQ from showing-user

The system SHALL keep HQ's awareness independent of user surfacing: gtmux SHALL
NOT force-type low-value (`routine`/QUIET) event lines into the HQ pane as a way
to inform HQ. HQ's full awareness comes from the session journal itself —
decision-dense events knock (the wake channel), and HQ pulls the complete delta
(`gtmux events --since-seq <n>`) on any knock; there is NO push copy of the
stream. HQ's awareness of an event SHALL be independent of whether the user is
shown anything about it.

#### Scenario: Low-value events reach HQ silently

- **WHEN** a QUIET-tier event (e.g. a resolved wait, a send-landed confirmation)
  occurs while an HQ pane is live
- **THEN** no visible `» gtmux·<class>` line is typed for it, and HQ receives it
  in its next pulled delta

#### Scenario: HQ omniscience is decoupled from user surfacing

- **WHEN** any event occurs while an HQ pane is live
- **THEN** the event is retained in the journal and delivered through HQ's next
  pull regardless of surfacing tier, and only a CRITICAL/NORMAL judgment by HQ
  produces user-visible output

### Requirement: Zero-loss cursor catch-up and gap detection

The pull read SHALL be seq-exact and self-auditing at the moment of consumption:
`gtmux events --since-seq <n>` SHALL return exactly the retained events with
sequence greater than the cursor, oldest first, and SHALL detect a gap — the
first returned event not being cursor+1, or a hole inside the returned tail —
every time it reads. On a gap it SHALL warn the reader (one CRITICAL line,
separate from the record output so `--json` consumers stay parseable) to rebuild
from a full `gtmux digest` snapshot before acking, rather than letting the
reader proceed as if nothing was missed. A cursor of 0 ("everything retained")
never reports a leading gap.

#### Scenario: Restart replays missed events

- **WHEN** HQ last acked sequence N and pulls `--since-seq N` while the journal
  has advanced to N+K
- **THEN** the read returns the events with sequence in (N, N+K] and none twice

#### Scenario: A gap triggers reconciliation

- **WHEN** HQ pulls `--since-seq N` and the first retained event after N is not
  N+1 (events rotated away), or the returned tail has a hole
- **THEN** the output carries one CRITICAL warning telling HQ to reconcile from
  a full digest snapshot before acking — at the moment of the read, not at some
  daemon's startup

#### Scenario: A contiguous read stays clean

- **WHEN** HQ pulls `--since-seq N` and the retained tail is contiguous from N+1
- **THEN** no gap warning is printed

### Requirement: Degradation is surfaced as CRITICAL

The system SHALL surface a perception-layer degradation to the user as a
CRITICAL condition immediately. Two degradations remain: a broken WAKE channel
(knocks queued but unconfirmed — the `wake-degraded` control record appended to
the session journal at `important` severity, plus one visible HQ-pane nudge and
a desktop notification), and a RETENTION overrun (events rotated away unread —
the read-time gap warning of the catch-up requirement). Recovery SHALL clear
the degradation state without re-alerting on the recovery.

#### Scenario: An outage is announced at once

- **WHEN** wake deliveries stop landing on a live HQ pane
- **THEN** a `wake-degraded` control record lands in the journal (counting
  toward the consumption debt), one visible nudge reaches the HQ pane, and a
  desktop notification fires

#### Scenario: Recovery does not re-alert

- **WHEN** a previously degraded wake channel becomes healthy again
- **THEN** the degradation state clears and no new alert fires for the recovery
  itself
