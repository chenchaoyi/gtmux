# Mobile App Specification

## Purpose

A phone app (the third surface) to monitor your tmux coding agents remotely, get
lock-screen push when one needs you, and — gated only by the pairing bearer token —
type back into a pane to unstick or steer an agent. Its look mirrors the menu-bar
app, so all three surfaces read as one product.
## Requirements
### Requirement: Pair with a Mac

The system SHALL let the user pair a Mac by host+token (and, later, a scanned
pairing QR), validating reachability + token before saving the pair to the device
Keychain.

#### Scenario: Manual pairing

- **WHEN** the user enters the Mac's reachable host and token and connects
- **THEN** the app verifies `/api/health` + an authed call, saves the pair to the
  Keychain, and shows the Radar; a failure gives a plain reachability diagnosis

### Requirement: Mirror the status language

The system SHALL render agents with the status language identical to the menu-bar
app — authoritative status colors, the same shapes+glyphs (waiting red square+
pause, working cyan static ring, idle green check, running gray dot), and the
fixed section order waiting→working→idle→running.

#### Scenario: Radar matches the menu bar

- **WHEN** the Radar shows agents
- **THEN** their colors, shapes, glyphs, sectioning, and `primary`/`secondary`
  row text match the menu-bar app

### Requirement: Live radar via SSE

The system SHALL load agents from `/api/agents` and refetch on the `agents` SSE
event, refetch immediately when the app returns to the FOREGROUND (iOS suspends the
SSE stream while backgrounded, so the cached list would otherwise be stale until a
manual pull-to-refresh), show an in-app banner on a foreground `alert`, and reflect
connection state. `/api/agents` is the only data source.

#### Scenario: Live update

- **WHEN** an agent's status changes on the Mac
- **THEN** the Radar updates via the SSE-triggered refetch

#### Scenario: Refresh on foreground

- **WHEN** the app returns to the foreground after being backgrounded
- **THEN** it immediately refetches `/api/agents` so the list is current, independent
  of the (suspended) SSE stream

### Requirement: Detail with terminal + chat views

The system SHALL show a selected agent's Detail in two switchable views kept fresh:
a "终端/terminal" view rendering the pane's live screen via the native pane renderer
(see `mobile-pane-renderer`), and a "对话/chat" view rendering the parsed transcript
(see `mobile-chat-view`, fed by `/api/transcript`). (A phone-side "focus on Mac"
action was removed in #85 — it has little value when you are remote; the `/api/focus`
endpoint stays for the browser mirror + as a stable contract.)

#### Scenario: Terminal view

- **WHEN** the user opens an agent's Detail terminal view
- **THEN** the pane's live screen is rendered (colors, cursor, long-press copy) and
  kept fresh

#### Scenario: Chat view

- **WHEN** the user switches Detail to the chat view
- **THEN** the parsed transcript is shown as a conversation and kept fresh

### Requirement: Push registration + tap deep-link

The system SHALL, when paired and push is enabled, request notification
permission, register the APNs device token to the Mac, and deep-link a tapped
notification to that agent's Detail (including cold start).

#### Scenario: Tap a push

- **WHEN** the user taps a delivered push carrying a `pane`
- **THEN** the app opens to that agent's Detail

### Requirement: Terminal input, gated by the pairing token

The system SHALL let the user type into a pane — literal text (optionally + Enter),
named control keys, and the waiting-pane `1/2/3` approval choices — via
`POST /api/send`, gated ONLY by the pairing bearer token (no separate
authorization), so the token must be treated as a password. After a send, the app
SHALL refresh the pane promptly (not wait for the next poll) so the user sees the
effect of their input quickly; it MAY optimistically echo a sent prompt.

#### Scenario: Send text

- **WHEN** the user types a message in the composer and sends it
- **THEN** the text is delivered via `/api/send` and the pane refreshes promptly to
  show the result

#### Scenario: Answer an approval

- **WHEN** a pane is waiting on a numbered prompt and the user taps a choice
- **THEN** the bare digit is sent via `/api/send` **without a trailing Enter** (the
  agent's numbered menus commit on the digit alone; a trailing Enter would leak onto
  the next prompt and auto-confirm it on consecutive selections) and the pane
  refreshes promptly
- **AND** the choices are presented as a compact row of number chips (`1..N`), not
  re-sketched label rows — the labels are already visible in the terminal/chat

### Requirement: Mobile shows native sessions in an "Elsewhere" section
The mobile app SHALL group `source: "native"` sessions into their own "Elsewhere / 不在 tmux" section, separate from the tmux status groups. These rows are sense-only: they carry a `native` tag, no jump chevron, and tapping one SHALL NOT open a terminal mirror (there is none). Moving a native session into tmux stays a menu-bar/CLI action; the mobile app is display-only for the native category.

#### Scenario: Native section on mobile
- **WHEN** the phone polls the radar and native sessions are present
- **THEN** they SHALL appear in a dedicated "Elsewhere" section, marked non-tappable (no terminal), distinct from the tmux groups

#### Scenario: Tapping a native row does nothing
- **WHEN** the user taps a native row on mobile
- **THEN** the app SHALL NOT navigate to a terminal/detail view for it

### Requirement: Mark errored idle rows in the mobile radar

The mobile radar SHALL visually distinguish an idle agent that ended on an error
(`error: true` in the `agents --json` contract) from a successfully-finished idle
agent, using an amber ⚠ "errored" modifier and the `error_text` summary in place of
the green ✓. The row SHALL remain in the idle section and MUST NOT use the red
`waiting`/needs-you color.

#### Scenario: Errored idle agent

- **WHEN** an agent row has `status: idle` and `error: true`
- **THEN** the mobile radar renders it in the idle section with an amber ⚠ marker
  (not the green ✓) and surfaces the `error_text` summary
- **AND** it is not colored red and does not sort into the needs-you section

#### Scenario: Successful idle agent unchanged

- **WHEN** an agent row has `status: idle` without `error`
- **THEN** the mobile radar renders it exactly as today (green ✓)

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

### Requirement: The supervisor opens a HQ command center, not the generic detail

When the user opens a `role:"supervisor"` session on mobile, the app SHALL
present a dedicated HQ command center — NOT the generic Chat/Terminal detail. It
SHALL contain: a status strip (fleet counts + subscription-window %), a fleet
board listing every agent from `/api/digest` (needs-you first; each row shows
state, location, agent, goal, and — when waiting — its ask), and a command
console (a conversation with the supervisor plus a command bar with free text
and quick-command chips). Commands are HQ-mediated: the command bar addresses the
supervisor, which drives the fleet; the HQ screen has NO
direct-send input; direct control lives in each worker's own detail, reached by
long-pressing a fleet row.

#### Scenario: Open the supervisor

- **WHEN** the user taps the gtmux HQ card (a `role:"supervisor"` row)
- **THEN** the HQ command center opens with the fleet board + command console,
  not the generic Chat/Terminal segmented detail

#### Scenario: Sense the fleet and command through HQ

- **WHEN** the user is in the HQ command center
- **THEN** the fleet board reflects all agents from `/api/digest`, and a message
  (typed or a quick-command chip) is delivered to the supervisor session

#### Scenario: Selecting a fleet row targets a command

- **WHEN** the user taps a fleet row
- **THEN** it is selected and per-target quick actions (e.g. continue / inspect /
  reply-for-me) become available in the command bar; a long-press instead jumps
  to that worker's own detail

