# supervisor-agent (delta)

## MODIFIED Requirements

### Requirement: Signal register separates wake traffic from conversation

The seeded playbook SHALL mandate two output registers: replies to wake lines use
the SIGNAL register — one line opening with `⟣` and a fixed glyph vocabulary
(✅ done-judgment, ▪ noted-to-board, 📓 captured-to-KB, ◈ tick brief, ⚠ escalation),
with at most two indented detail lines (tick briefs ≤6 lines total) — while replies to the
human use ordinary conversational prose with no sigils. The `⟣ 📓 captured: <topic-file>`
line SHALL be the capture verdict named in the capture-verify requirement and SHALL be
emitted ONLY on a REAL capture (never as an empty "considered it" marker). Wake turns
SHALL be short: pull the delta, judge, capture (when a forced class or a real
opportunistic fact warrants it), update the board, emit the signal line; no narration.

The register vocabulary SHALL map onto the wake channel's three attention grades
(decision · attention · ledger), and the playbook SHALL gate PRINTING by grade:
decision-grade output leads with the decision glyph and must be printable at a glance;
attention-grade output is the normal one-line signal; LEDGER-GRADE content — pure
bookkeeping with no interrupt value — goes to the board/ledger and is NOT printed to the
screen (measured motivation: ~70% of one shift's printed output was bookkeeping the
commander had to scan past to find the ~5% needing a decision). This composes with the
existing quiet-threshold gate; grade decides WHETHER a line prints, the register decides
HOW it looks.

#### Scenario: A done wake gets a one-line judgment

- **WHEN** HQ processes a `done` wake for a session that completed its goal
- **THEN** its reply is a single `⟣ ✅ …` line (judgment + suggested next step),
  visually distinct from conversation

#### Scenario: A capture is marked in the register

- **WHEN** HQ folds a durable lesson from a forced-class closure into a KB topic file
- **THEN** it emits a `⟣ 📓 captured: <topic-file>` line, in the signal register, never
  mixed with human prose

#### Scenario: A trivial done is noted silently

- **WHEN** HQ judges a done wake to be an unremarkable intermediate completion
- **THEN** it updates the board and replies with at most one `⟣ ▪` note line, and emits no
  `⟣ 📓` (capture is opportunistic on done, not forced)

#### Scenario: Bookkeeping does not print

- **WHEN** HQ's reply to a wake would carry only ledger-grade content (recorded, nothing
  blocked, nothing to decide)
- **THEN** the content goes to the board/ledger and the screen gets at most one minimal
  register line, never a prose block

### Requirement: Periodic tick brief

On a `tick` wake the playbook SHALL direct HQ to pull the covered delta, update
the situation board, and emit ONE brief in the signal register — at most six
lines: a `⟣ ◈` headline with fleet counts and the top item needing attention,
then up to five indented outcome lines (completions with a one-clause summary,
new sessions, stalls). The brief SHALL respect the resolved quiet threshold; in
quiet mode board-only unless something CRITICAL rode the tick.

The brief SHALL report the DELTA since the last brief, not a snapshot: only items that
changed (completed, appeared, stalled, moved state) are named. The standing
"on your plate" set SHALL NOT be re-printed when unchanged — it lives in the
pending-decision standing view (see hq-attention-system), and the brief references it in
at most one clause (e.g. a count, or "plate unchanged"). A plate item IS named when it
changed this interval (entered, resolved, or escalated).

#### Scenario: The brief is bounded and concrete

- **WHEN** a tick wake covers three completions and one new session
- **THEN** HQ emits one ≤6-line `⟣ ◈` brief naming each outcome in one clause, and
  nothing else

#### Scenario: An unchanged plate is a pointer, not a reprint

- **WHEN** a tick interval passes in which the pending-decision set did not change
- **THEN** the brief does not re-list the pending items — at most one clause references
  the standing view — and a brief in which nothing at all changed is not emitted (the
  zero-change gate already suppresses the tick)
