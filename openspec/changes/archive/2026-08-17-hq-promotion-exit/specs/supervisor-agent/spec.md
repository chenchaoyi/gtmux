# supervisor-agent (delta)

## MODIFIED Requirements

### Requirement: Correction-to-charter learning loop

The seeded playbook SHALL make learning from corrections a FIRST-CLASS ritual, not an ad-hoc
afterthought: when the commander CORRECTS HQ, or the SAME footgun is hit more than once, HQ
SHALL distill the durable lesson and land it — a PORTABLE behavior lesson into the
`best-practices` or `pitfalls` knowledge topics (through the knowledge verbs), and, when the
lesson is CHARTER-LEVEL (it belongs in gtmux's seeded playbook, specs, or code — the
promotion test: it holds on another machine AND changes how gtmux itself should behave),
PROMOTE it: `gtmux knowledge promote <id> --why "…" [--target "…"]` writes the promotion
brief that IS the exit, and `gtmux knowledge land <id> --ref "…"` closes the loop when it
lands in the repo. A local flags list is NOT the mechanism — an un-carried flag rots and
drifts (measured: 34 accumulated items, and an estimate off by ~50× by the time it was
audited). A MACHINE-SPECIFIC instance goes into local notes. The playbook SHALL state the
trigger points (a commander correction; a repeated footgun) and the landing path explicitly,
so HQ actually self-upgrades from the interaction, and SHALL direct HQ to migrate any
pre-existing local flags backlog through the promotion verbs — judged entry-by-entry, never
bulk-imported. The knowledge scaffold SHALL include a `corrections` topic as the landing
place for distilled corrections.

#### Scenario: A correction is distilled and landed

- **WHEN** the playbook covers the commander correcting HQ, or a footgun recurring
- **THEN** it directs HQ to distill the lesson into the knowledge base (portable) or local
  notes (machine-specific) and to PROMOTE a charter-level lesson through
  `gtmux knowledge promote`, closing with `land` when it reaches the repo

#### Scenario: The scaffold has a corrections topic

- **WHEN** `gtmux hq` seeds the knowledge scaffold
- **THEN** a `corrections` topic exists and the KB README lists it

#### Scenario: A flags file is not an exit

- **WHEN** HQ holds charter-level lessons only in a local notes file
- **THEN** the playbook directs it to promote the ones that still hold (and dismiss or
  retire the rest), so the queue — not the file — carries them

### Requirement: The seeded playbook teaches the knowledge-distillation ritual

The seeded playbook SHALL teach HQ, on a gtmux-raised `distill` trigger — delivered as a
STANDING-priority wake line (`» gtmux·distill …`) and recorded as a
`[CONTROL gtmux:distill]` entry in the session-event stream — to run a retrospective
knowledge-distillation pass: read the fleet's event/outcome delta since the last distill,
drain the pending-distill spool CANDIDATE BY CANDIDATE (`gtmux knowledge add --capture
<key>` to accept one with its provenance, `gtmux knowledge dismiss --capture <key> --why`
to reject one with a trace), fold durable cross-cutting facts into the right topic
through the knowledge verbs (`add` for a new lesson, `supersede` for one that replaces an
existing entry — the mechanical form of update-over-append), PRUNE stale or dead entries
with `retire --why`, and migrate any legacy-file lesson the pass touches into the ledger.
The pass SHALL also check `gtmux knowledge promotions`: a pending brief past its
staleness floor is escalation material for the commander, because a promotion nobody
carries is silent rot. The ritual SHALL be distinct from `self-check` (own-artifact
health housekeeping) and `tick` (the user-facing summary brief). HQ SHALL default to
SILENT distillation, printing a one-line brief ONLY when it made a real curation; a
charter-level lesson SHALL be PROMOTED (`gtmux knowledge promote`) rather than only
noted locally; and the never-store-secrets rule SHALL continue to apply. Because the
trigger is also a stream record, the playbook SHALL teach that a distill missed on the
wake channel is recoverable by PULL (`gtmux events --since-seq`) rather than lost. The
shipped playbook version SHALL be bumped so existing HQ homes adopt the ritual on their
next managed-playbook upgrade.

#### Scenario: Silent when nothing durable accrued

- **WHEN** a `distill` trigger fires and the period's delta yields no durable new fact,
  no stale entry, and no duplicate to merge
- **THEN** HQ performs the pass and prints nothing

#### Scenario: A real distillation is briefed in one line

- **WHEN** a `distill` trigger fires and HQ folds a recurring cross-session fact into a
  topic through the knowledge verbs and retires a dead entry
- **THEN** HQ prints a single one-line brief of what it curated

#### Scenario: The distill delta is not a duplicate of moment-capture

- **WHEN** a durable fact was already captured in the knowledge base the moment it was
  learned
- **THEN** the distill pass SUPERSEDES that entry rather than adding a second copy,
  because it works the delta since the watermark and consolidates rather than
  re-summarizes

#### Scenario: Secrets are never distilled into the base

- **WHEN** the period's activity includes a password, token, or private key
- **THEN** the distillation records only IDs / methods / pointers, never the secret

#### Scenario: A missed knock is recoverable by pull

- **WHEN** a `distill` wake line does not reach the HQ pane (queue eviction, a wake
  outage) but the trigger was raised
- **THEN** the `[CONTROL gtmux:distill]` record is still in the event stream, so HQ's next
  delta pull surfaces the pending pass instead of it being silently lost

#### Scenario: A stale promotion is escalated, not forgotten

- **WHEN** a distill pass finds a promotion pending past its staleness floor
- **THEN** HQ raises it to the commander instead of letting the queue rot silently
