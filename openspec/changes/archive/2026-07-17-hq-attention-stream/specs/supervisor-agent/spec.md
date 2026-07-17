# supervisor-agent — delta

## MODIFIED Requirements

### Requirement: Query the attention stream, not raw transcripts

The seeded playbook SHALL direct the supervisor to triage from the event stream and the
digest — the per-record `summary` for what was said — and NOT to read raw transcripts
line-by-line, which doubles token cost.

It SHALL name the THREE reads for what they are, and SHALL NOT present any one of them as
"the attention stream":

- `gtmux events --since-seq <n>` (unfiltered) — the DELTA since HQ's cursor: what a wake
  tells HQ to run, and the reconcile path whenever HQ doubts its picture;
- `gtmux events --severity notable` — the FLEET-CHANGE stream: instructions reaching
  sessions, turn-ends, lifecycle;
- `gtmux events --severity important` — the ESCALATION stream: blocked, asking, crashed.
  A subset to triage FIRST, never the whole picture.

The playbook SHALL state the rule that generalizes: a filtered read is a triage shortcut,
NOT HQ's model of the world — every filter is a claim about what does not matter, so HQ
reconciles with the unfiltered delta (or the digest) rather than trusting one tier. The
Toolbox section SHALL document `gtmux events --severity` with that framing.

#### Scenario: The playbook names the three reads

- **WHEN** the HQ home is seeded
- **THEN** the playbook describes the unfiltered `--since-seq` delta as the reconcile
  path, `--severity notable` as the fleet-change stream, and `--severity important` as the
  escalation subset — and instructs HQ to record summaries rather than raw transcripts

#### Scenario: A filtered read is never the whole picture

- **WHEN** the playbook covers triage from a severity filter
- **THEN** it states that a filter is a shortcut rather than HQ's model of the world, and
  points at the unfiltered delta (or the digest) for reconciling

#### Scenario: The user's direct instruction is visible by pull

- **WHEN** the user submits an instruction directly into a non-HQ agent pane and HQ later
  catches up by pull rather than by the wake line
- **THEN** the instruction is in the stream HQ is told to read (`notable` and above), not
  filtered out as routine chatter
