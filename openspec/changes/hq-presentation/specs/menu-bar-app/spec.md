## ADDED Requirements

### Requirement: The supervisor renders as its own layer (HQ card)

The popover SHALL render a supervisor session (`role:"supervisor"`) as a
persistent compact card between the header summary and the grouped section list
— NEVER as a row inside the waiting/working/idle/running sections (those rows
SHALL exclude supervisor rows). The card carries the gtmux brand pane-grid mark
as its avatar (the supervisor is gtmux's own concept — visually distinct from
agent avatars), the standard status badge, and the session's task line; clicking
it focuses the supervisor's pane. When no supervisor is live, the slot SHALL
show a quiet "not running — start" affordance that launches `gtmux hq` (the app
stays a CLI consumer).

#### Scenario: Supervisor live

- **WHEN** an `agents --json` row carries `role:"supervisor"`
- **THEN** the popover shows the HQ card (brand mark + status badge + task) above
  the sections, and that row does NOT appear inside any section

#### Scenario: Supervisor absent

- **WHEN** no row carries `role:"supervisor"`
- **THEN** the HQ slot shows the quiet start affordance, and clicking it shells
  `gtmux hq`
