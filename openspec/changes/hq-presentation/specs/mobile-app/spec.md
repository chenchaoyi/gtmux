## ADDED Requirements

### Requirement: The supervisor renders as its own layer (HQ card)

The radar SHALL render a supervisor session (`role:"supervisor"`) as a compact
card below the server chip — NEVER as a row inside the status sections (the
section grouping SHALL exclude supervisor rows). Tapping the card opens the
supervisor's Detail in CHAT mode (conversing with the supervisor is the primary
mobile path). When no supervisor is live the card is simply absent (starting one
requires the Mac; the phone shows no dead control).

#### Scenario: Supervisor live on mobile

- **WHEN** `/api/agents` includes a `role:"supervisor"` row
- **THEN** the radar shows the HQ card below the server chip, the row is excluded
  from the sections, and tapping the card opens its Detail in chat mode

#### Scenario: Supervisor absent on mobile

- **WHEN** no row carries `role:"supervisor"`
- **THEN** no HQ card (and no dead "start" control) is shown
