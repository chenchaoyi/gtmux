# hq-attention-system Specification

## Purpose
TBD - created by archiving change hq-attention-system. Update Purpose after archive.

## Requirements

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

### Requirement: Pending-decision standing view

The ledger SHALL support a first-class "awaiting-commander" disposition, and
the system SHALL provide a standing view of exactly those entries (`gtmux tasks
--pending` — a filter on the existing ledger surface, no new top-level command), with
mark / unmark surfaces on the same command (`--await <id>` / `--resolve <id>
[disposition]`). The view SHALL be the single home of "what is on the commander's
plate": stable ordering, en+zh, and content that changes ONLY when an entry enters,
leaves, or changes state in the pending set. Briefs and reports reference this view
instead of re-printing the list (measured motivation: every brief of a 400+-turn shift
re-printed the same unchanged 「你手上未变: A · B · C…」 list — a persistent view
simulated in a scrolling transcript).

Each entry SHALL carry the free-text `disposition` and the `awaiting-since` /
first-seen / last-update timestamps — additive and optional, so a legacy entry
(including one written when the ledger carried tier/priority/surfaced/archive
fields) still loads. `gtmux tasks --verbose` SHALL add the disposition detail
to the live rows.

The ordering SHALL be TOTAL — the oldest wait first, then pane, then id — so
two reads of an unchanged set cannot differ. Every row renders at attention
grade (`▸`): an entry on the plate is a thing awaiting a decision, which is
not bookkeeping. The view
SHALL NOT render a relative clock (a countdown would churn the output the view
exists to stabilize) and SHALL NOT require a radar scan (it is polled, and the
plate is not a question about pane state).

Membership SHALL be the disposition and nothing else, so setting any other
disposition also removes an entry. The disposition is additive: a legacy
ledger entry without it still loads, none is pending, and marking one requires
no migration.

#### Scenario: The plate has one home

- **WHEN** an attention item is marked awaiting-commander
- **THEN** it appears in the pending view, and remains there unchanged-in-place until its
  state changes (decided, withdrawn, escalated)

#### Scenario: Two reads of an unchanged plate are identical

- **WHEN** the pending view is read twice with no entry entering, leaving, or changing
- **THEN** the two outputs are byte-identical

#### Scenario: Legacy entries are unaffected

- **WHEN** a ledger written before this change is loaded — with or without the
  retired tier/priority/surfaced/archive fields
- **THEN** every entry loads, none is in the pending set, and marking one
  awaiting-commander requires no migration

### Requirement: Attention grades render distinctly on gtmux-owned surfaces

Surfaces gtmux itself renders — at minimum `gtmux events --follow` and the tasks/ledger
views — SHALL render each row's attention grade (decision · attention · ledger, the same
three-value scale the wake lines carry) distinctly: in color when stdout is a tty,
suppressed under `NO_COLOR`, and byte-identical to today's output when not a tty (pipes
and scripts see no change). Color encodes ONLY the grade — consistent with the design
rule that color carries exactly one meaning per surface. The agent-facing pane content
is out of scope here: the composer and agent TUIs cannot carry ANSI color, which is why
the wake line's grade is a glyph (see hq-wake-protocol).

#### Scenario: A tty read scans by grade

- **WHEN** `gtmux events --follow` runs on an interactive terminal
- **THEN** decision-grade rows are visually distinct from attention- and ledger-grade
  rows by color, and the glyph/text content is identical to the uncolored form

#### Scenario: Scripts see no change

- **WHEN** the same command's output is piped, or `NO_COLOR` is set
- **THEN** the bytes carry no escape sequences and match today's format

### Requirement: Surfacing configuration

The system SHALL let the user tune the surfacing threshold. `config.json` SHALL support a
`surfaceTier` (`critical`|`normal`|`quiet`, default `normal` = surface NORMAL and above)
and a `quiet` toggle equivalent to raising the threshold to CRITICAL-only. The system
SHALL provide `gtmux quiet [on|off|status]` as the front door, and SHALL expose the
resolved threshold so HQ gates its prints accordingly. The threshold SHALL NEVER suppress
a degradation CRITICAL.

#### Scenario: Quiet mode raises the bar

- **WHEN** `gtmux quiet on` is set
- **THEN** the resolved surfacing threshold becomes CRITICAL-only and NORMAL items are
  ledger-recorded without a user-visible print

#### Scenario: Default threshold matches today's attention level

- **WHEN** no surfacing config is set
- **THEN** the resolved threshold is NORMAL-and-above

#### Scenario: A degradation is never quieted

- **WHEN** the surfacing threshold is CRITICAL-only (quiet on) and a feed degradation
  occurs
- **THEN** the degradation is still surfaced (it is CRITICAL and cannot be suppressed)

### Requirement: Self-check triggers sensed by gtmux

The system SHALL sense, LLM-free in the slow-tick, when HQ should run a self-check and
raise a `self-check` trigger to HQ (delivered as a feed control record, not counted as
user-facing). A trigger SHALL be raised when: the machine has been idle ≥ ~2 h with no
CRITICAL/NORMAL surfaced AND ≥ ~12 h since the last self-check (the resting-user case); OR
a threshold trips (open ledger entries over a cap, the journal over its rotation ceiling,
or a cursor gap); OR a daily floor (≥ 24 h since the last self-check). Triggers SHALL be
rate-limited to at most one per hour.

#### Scenario: Resting-user idle trigger

- **WHEN** the machine has been idle ≥ ~2 h with nothing surfaced and it has been ≥ ~12 h
  since the last self-check
- **THEN** gtmux raises a self-check trigger to HQ

#### Scenario: Threshold trigger fires immediately

- **WHEN** the open ledger exceeds its cap or the journal exceeds its rotation ceiling
- **THEN** gtmux raises a self-check trigger without waiting for idle

#### Scenario: Rate limited

- **WHEN** conditions would raise a second self-check trigger within an hour of the last
- **THEN** no second trigger is raised until the hour has elapsed
