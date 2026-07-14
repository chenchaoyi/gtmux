# supervisor-agent Specification

## ADDED Requirements

### Requirement: HQ maintains a persistent situation board across context resets

The system SHALL seed a situation board at `~/.config/gtmux/hq/notes/board.md` — written
only when ABSENT (never clobbering HQ's curated content), the same write-when-absent
discipline as the knowledge scaffold — and the seeded playbook SHALL direct the supervisor
to keep it current as its durable command posture: one row per ship (agent) carrying its
task, command mode / dispatch source, priority, health, any pending decision, and the most
recent lesson. Because HQ's own context is periodically compacted or reset, the playbook
SHALL instruct HQ to re-read the board at the start of a turn after a reset — before acting —
so posture survives the reset, and to treat the deterministic `gtmux digest`/`tasks`/`events`
as the source of record while the board is HQ's synthesis. The board SHALL be HQ-curated
markdown, NOT a gtmux-parsed schema (gtmux does not read it back).

#### Scenario: A fresh home seeds the board

- **WHEN** `gtmux hq` seeds a home with no `notes/board.md`
- **THEN** a `board.md` template is created (per-ship task · mode/source · priority · health ·
  pending · lesson), and a subsequent run leaves HQ's curated board untouched

#### Scenario: Posture survives a context reset

- **WHEN** the seeded playbook covers HQ resuming after a `/compact` or context reset
- **THEN** it directs HQ to re-read `notes/board.md` before acting, rather than re-deriving
  the whole fleet from scratch

### Requirement: Query the attention stream, not raw transcripts

The seeded playbook SHALL direct the supervisor to triage from the SEVERITY-filtered event
stream and the digest — `gtmux events --severity important` for what needs attention, the
per-record `summary` for what was said — and NOT to read raw transcripts line-by-line, which
doubles token cost. The Toolbox section SHALL document `gtmux events --severity`.

#### Scenario: The playbook points triage at the filtered stream

- **WHEN** the HQ home is seeded
- **THEN** the playbook instructs HQ to read `gtmux events --severity important` and record
  summaries rather than raw transcripts, and the Toolbox lists `--severity`

### Requirement: Decision-authority tiers — when HQ decides versus escalates

The seeded playbook SHALL encode the commander's three interaction modes — ① dispatch a ship
directly, ② adopt HQ's suggestion, ③ discuss then let HQ decide and delegate — and an explicit
autonomy matrix for mode ③: HQ MAY decide-and-dispatch autonomously ONLY when the action is
REVERSIBLE **and** LOW-RISK **and** WITHIN AN ALREADY-DISCUSSED DIRECTION; HQ MUST escalate to
the commander when the action is IRREVERSIBLE, touches PERMISSIONS/CREDENTIALS, FORKS the
plan/approach, or falls OUTSIDE the discussed scope. This SHALL NOT loosen the existing rule
that HQ never answers another agent's permission/plan/design choice on the user's behalf —
it makes mode ③ concrete without granting HQ authority over the commander's decisions.

#### Scenario: A reversible in-scope action may be decided

- **WHEN** the playbook covers a reversible, low-risk action within a direction the commander
  already discussed (e.g. re-dispatching a follow-up the user asked to continue)
- **THEN** it permits HQ to decide and dispatch it, noting what it did and to whom

#### Scenario: An irreversible or forking action is escalated

- **WHEN** the action is irreversible, touches permissions/credentials, forks the
  plan/approach, or is outside the discussed scope
- **THEN** the playbook directs HQ to escalate to the commander rather than decide it

### Requirement: Graded escalation and reconcile-before-relay

The seeded playbook SHALL define GRADED escalation channels keyed on severity — `routine`
items update the situation board only (no interrupt); `important` items reach HQ as a
coalesced summary; `critical` conditions ensure the commander is pushed (via the existing
notification pipeline, which already surfaces attention events to the phone) — so only
genuinely critical conditions "ring". The playbook SHALL define `critical` as the runtime
judgment HQ layers over important events: quota near-exhaustion (from `gtmux limits`/`usage`),
a production/线上 issue, or one agent blocking others. The playbook SHALL further require a
RECONCILE step: before relaying or escalating any needs-you, HQ re-checks the LIVE
`gtmux digest`/`tasks` for that pane and DROPS the item if the state already moved (the pane
was answered directly, resumed, or finished) — eliminating stale needs-you false positives.
This complements the `resolved`-nudge retraction, covering the delayed/queued/post-reset case
where no `resolved` nudge was observed.

#### Scenario: Only critical conditions ring

- **WHEN** the playbook covers a routine turn-end versus a quota-near-exhaustion condition
- **THEN** it directs the routine item to the board silently and the critical one to a push,
  with `important` items coalesced into an HQ summary in between

#### Scenario: A stale needs-you is reconciled away

- **WHEN** HQ is about to relay a needs-you and the live digest shows that pane already left
  waiting (answered directly / resumed / finished)
- **THEN** the playbook directs HQ to reconcile against the live digest and DROP the relay
  rather than forward a stale one

### Requirement: Correction-to-charter learning loop

The seeded playbook SHALL make learning from corrections a FIRST-CLASS ritual, not an ad-hoc
afterthought: when the commander CORRECTS HQ, or the SAME footgun is hit more than once, HQ
SHALL distill the durable lesson and land it — a PORTABLE behavior lesson into
`knowledge/best-practices.md` or `knowledge/pitfalls.md` (and, when the lesson is
charter-level, FLAG it for a seed/spec update rather than only noting it locally); a
MACHINE-SPECIFIC instance into local notes. The playbook SHALL state the trigger points
(a commander correction; a repeated footgun) and the landing path explicitly, so HQ actually
self-upgrades from the interaction. The knowledge scaffold SHALL include a `corrections.md`
topic as the landing place for distilled corrections.

#### Scenario: A correction is distilled and landed

- **WHEN** the playbook covers the commander correcting HQ, or a footgun recurring
- **THEN** it directs HQ to distill the lesson into the knowledge base (portable) or local
  notes (machine-specific) and to flag a charter-level lesson for a seed/spec update

#### Scenario: The scaffold has a corrections topic

- **WHEN** `gtmux hq` seeds the knowledge scaffold
- **THEN** a `corrections.md` topic file exists and the KB README lists it

## MODIFIED Requirements

### Requirement: The supervisor curates a persistent knowledge base

The supervisor's primary long-term value SHALL be curating a living, cross-cutting
knowledge base under its home (`~/.config/gtmux/hq/knowledge/`). On first run the
system SHALL seed a scaffold — an index README plus topic files (accounts,
workflows, best-practices, pitfalls, environment, and corrections) — each written only when
ABSENT so the supervisor's curated content is never overwritten. The playbook SHALL direct
the supervisor to capture durable, reusable facts once, keep them current, consult
them before advising or driving, and iterate on them — and SHALL forbid storing
secrets (passwords, tokens, keys), recording only IDs, methods, procedures, and
pointers to where a secret lives.

#### Scenario: Knowledge scaffold seeded, never clobbered

- **WHEN** `gtmux hq` first runs (no `knowledge/` yet)
- **THEN** the scaffold (README + topic files, including `corrections.md`) is created; a
  subsequent run adds only missing files and leaves the supervisor's curated content untouched

#### Scenario: No secrets in the knowledge base

- **WHEN** the supervisor records account or service knowledge
- **THEN** its playbook requires IDs/methods/pointers only — never passwords,
  tokens, or private keys
