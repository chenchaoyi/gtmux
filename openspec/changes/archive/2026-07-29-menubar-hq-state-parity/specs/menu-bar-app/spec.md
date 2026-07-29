# menu-bar-app Specification

## MODIFIED Requirements

### Requirement: The supervisor renders as its own layer (HQ card)

The popover SHALL render a supervisor session (`role:"supervisor"`) as a
persistent compact card between the header summary and the grouped section list
— NEVER as a row inside the waiting/working/idle/running sections (those rows
SHALL exclude supervisor rows). The card SHALL be visually framed so it does NOT
read as one more session: a ROLE BANNER above it (a small uppercase
"CHIEF OF STAFF / 参谋长" label with an oversight glyph and a short purpose line,
e.g. "watches all sessions / 统观全局") and a BORDERED panel (agent rows carry no
border — the border is the primary "not a row" cue).

The card's avatar SHALL be a circular **HQ medallion** — the gtmux brand pane-grid
mark plus an "HQ" wordmark inside a ring — that is the SAME visual token as the mobile
HQ disc (MOBILE §17), so the supervisor reads as one identity across surfaces. The
medallion's RING COLOR and a small corner BADGE SHALL encode the full HQ state model,
in parity with the mobile disc and resolved by the same priority order (a pure resolver
mirroring the mobile `discState`):

- **needs-your-call** — the supervisor itself is `waiting` → RED ring + a `!` badge.
- **worker-needs-you** — ≥1 non-supervisor session is `waiting` → RED ring + the waiting
  COUNT as the badge.
- **resource-bottleneck** — the machine is at the `red` resource tier (a genuine
  bottleneck; from a slow `gtmux resource --json` poll of `machine.tier`, NOT a soft
  amber) → RED ring + a `⚠` badge.
- **working** — the supervisor is `working` → CYAN ring, no badge.
- **normal** — all quiet → GREEN ring, no badge.

RED is reserved for "needs attention" (a decision or a genuine bottleneck); the badge
disambiguates which. The ring/badge colors SHALL be the authoritative status palette
(DESIGN §9 / `Theme.Status`), the same values the section badges use — the medallion
adds NO new colors. The card SHALL still carry the deterministic INTELLIGENCE HEADLINE
as its subtitle (the chief-of-staff conclusion: who needs you + how many others are
normal, or "all normal"); the ring/badge is the at-a-glance layer and the headline is
the sentence. Clicking the card focuses the supervisor's pane (the command console
lives on mobile/web, not the menu bar).

When no supervisor is live, the slot SHALL show a quiet grey (dimmed) medallion with a
"not running — start" affordance that launches `gtmux hq` (the app stays a CLI
consumer).

#### Scenario: Supervisor live

- **WHEN** an `agents --json` row carries `role:"supervisor"`
- **THEN** the popover shows the HQ card (the HQ medallion + intelligence headline)
  above the sections, and that row does NOT appear inside any section

#### Scenario: The medallion ring encodes state in parity with the mobile disc

- **WHEN** the supervisor is `working` and nothing is waiting and the machine is not at
  the red tier
- **THEN** the medallion ring is CYAN with no badge
- **AND WHEN** a non-supervisor session is `waiting`, the ring is RED with the waiting
  count as the badge
- **AND WHEN** the supervisor itself is `waiting`, the ring is RED with a `!` badge,
  outranking a waiting worker and a resource bottleneck

#### Scenario: A soft resource amber does not redden the medallion

- **WHEN** `gtmux resource --json` reports `machine.tier` = `amber` (a soft heads-up,
  not a bottleneck) and nothing is waiting
- **THEN** the medallion stays on the supervisor's own state (working / normal) — only
  a `red` tier drives the resource state (低噪, matching the mobile disc)

#### Scenario: Supervisor absent

- **WHEN** no row carries `role:"supervisor"`
- **THEN** the HQ slot shows the quiet grey medallion start affordance, and clicking it
  shells `gtmux hq`
