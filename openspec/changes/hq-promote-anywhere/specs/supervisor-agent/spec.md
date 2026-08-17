# supervisor-agent (delta)

## MODIFIED Requirements

### Requirement: Correction-to-charter learning loop

The seeded playbook SHALL make learning from corrections a FIRST-CLASS ritual, not an ad-hoc
afterthought: when the commander CORRECTS HQ, or the SAME footgun is hit more than once, HQ
SHALL distill the durable lesson and land it — a PORTABLE behavior lesson into the
`best-practices` or `pitfalls` knowledge topics (through the knowledge verbs), and, when the
lesson is CHARTER-LEVEL (it holds on another machine AND belongs in a DURABLE RULE CARRIER
beyond this machine's knowledge base — a project's `AGENTS.md`/`CLAUDE.md`, a team runbook,
`LOCAL.md` when it governs this supervisor itself, or gtmux's own playbook/specs/code),
PROMOTE it: `gtmux knowledge promote <id> --why "…" [--target "…"]` writes the promotion
brief that IS the exit, and `gtmux knowledge land <id> --ref "…"` closes the loop when it
lands in its carrier — the ref naming a PR, an issue, a runbook, or a file alike. A local
flags list is NOT the mechanism — an un-carried flag rots and drifts (measured: 34
accumulated items, and an estimate off by ~50× by the time it was audited). A
MACHINE-SPECIFIC instance goes into local notes. The playbook SHALL state the trigger
points (a commander correction; a repeated footgun) and the landing path explicitly, so HQ
actually self-upgrades from the interaction, and SHALL direct HQ to migrate any
pre-existing local flags backlog through the promotion verbs — judged entry-by-entry, never
bulk-imported. The knowledge scaffold SHALL include a `corrections` topic as the landing
place for distilled corrections.

#### Scenario: A correction is distilled and landed

- **WHEN** the playbook covers the commander correcting HQ, or a footgun recurring
- **THEN** it directs HQ to distill the lesson into the knowledge base (portable) or local
  notes (machine-specific) and to PROMOTE a charter-level lesson through
  `gtmux knowledge promote`, closing with `land` when it reaches its carrier

#### Scenario: The scaffold has a corrections topic

- **WHEN** `gtmux hq` seeds the knowledge scaffold
- **THEN** a `corrections` topic exists and the KB README lists it

#### Scenario: A flags file is not an exit

- **WHEN** HQ holds charter-level lessons only in a local notes file
- **THEN** the playbook directs it to promote the ones that still hold (and dismiss or
  retire the rest), so the queue — not the file — carries them

#### Scenario: A non-developer's lesson has a legitimate landing

- **WHEN** a charter-level lesson governs the user's own fleet or projects rather than
  gtmux itself
- **THEN** the taught landing is the user's carrier (a project AGENTS.md, a team runbook,
  LOCAL.md) — never a mandate to edit gtmux's source tree
