# session-events — delta

## MODIFIED Requirements

### Requirement: Events carry a deterministic severity tier

Every event record SHALL carry an additive `severity` field classifying the event's
attention level as `routine`, `notable`, or `important`, computed by a DETERMINISTIC,
LLM-free classifier from fields the record already holds (event/state/kind/class/origin)
and stamped at the SOURCE (the single append path), so it is persisted and queryable
without recompute.

Severity ranks URGENCY — how much someone is waiting on the supervisor — not relevance.
A `Waiting` event (the pane needs the user) and a `Stop` classified `asking` (a reply-text
question) SHALL be `important`; a `Stop` classified `report`, the session lifecycle events
(`SessionStart`/`SessionEnd`/`Resumed`/`PreCompact`), and a prompt submission carrying an
INSTRUCTION SHALL be `notable`; notifications, ordinary working ticks, and a submission
that carries no instruction SHALL be `routine`.

A prompt submission SHALL carry an additive `origin` field marking whether its payload is
a real instruction — typed prose or a slash command — as opposed to harness-injected
content or gtmux's own wake line echoed back. It SHALL be stamped from the SAME classifier
that decides whether the submission wakes HQ, so the wake and the tier can never disagree
about what a user act is. It SHALL NOT claim WHO authored the instruction: a task the
system dispatched carries one too, and no reliable signal distinguishes them (an
instruction reaching a session is a fleet change either way).

Both fields are additive to the stable event contract — a record without them (a legacy
line) SHALL read as `routine` with no instruction. Stamping SHALL NOT alter the existing
marker/notify state machines, and the write path SHALL remain fire-and-forget so a busy or
absent consumer never blocks the hook.

#### Scenario: A waiting event is important

- **WHEN** the hook appends a `Waiting` event (the pane blocked on the user)
- **THEN** the record's `severity` is `important`

#### Scenario: An asking turn-end is important, a report turn-end is notable

- **WHEN** a `Stop` event is classified `asking` versus `report`
- **THEN** the former's `severity` is `important` and the latter's is `notable`

#### Scenario: A submitted instruction is a fleet change, not chatter

- **WHEN** a `UserPromptSubmit` carrying typed prose or a slash command is appended
- **THEN** its `origin` is the instruction marker and its `severity` is `notable`, so a
  supervisor reading the fleet-change stream sees what the user told a session to do

#### Scenario: Injected content is not an act

- **WHEN** a `UserPromptSubmit` whose payload is harness-injected content (or gtmux's own
  wake line echoed back) is appended
- **THEN** it carries no `origin` and its `severity` is `routine`

#### Scenario: A legacy record without severity reads as routine

- **WHEN** a record written before this field is read back
- **THEN** it is treated as `routine` for severity purposes, without failing the read

### Requirement: Severity-filtered event read

`gtmux events` SHALL accept `--severity <level>` (`routine`|`notable`|`important`) to
restrict the stream to events at that level AND ABOVE (`routine` < `notable` < `important`),
applied to BOTH the bare recent-window form and `--follow`. An unrecognized level SHALL be
rejected with the usage message.

The command's own help SHALL NOT present any filtered read as "the attention stream".
There are three reads and they SHALL be described as what they are: the unfiltered delta
(`--since-seq <n>`) is the reconcile path; `--severity notable` is the fleet-change stream
(instructions, turn-ends, lifecycle); `--severity important` is the ESCALATION stream
(blocked, asking, crashed) — a subset, never the whole picture. Together with the
per-record `summary`, none of them requires reading a raw transcript to triage.

#### Scenario: Filter to escalation-worthy events

- **WHEN** `gtmux events --severity important` runs over a stream mixing routine and
  important records
- **THEN** only the `important` records are printed

#### Scenario: Level is inclusive-and-above

- **WHEN** `gtmux events --severity notable` runs
- **THEN** both `notable` and `important` records are printed, and `routine` ones are omitted

#### Scenario: The help does not oversell a filter

- **WHEN** `gtmux events --help` describes `--severity`
- **THEN** it names the escalation stream as a subset and points at the unfiltered
  `--since-seq` delta for reconciling, rather than calling any filter "the attention
  stream"

#### Scenario: An invalid level is rejected

- **WHEN** `gtmux events --severity bogus` runs
- **THEN** the command reports the usage message rather than printing an unfiltered stream
