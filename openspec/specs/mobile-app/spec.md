# Mobile App Specification

## Purpose

A phone app (the third surface) to monitor your tmux coding agents remotely and
get lock-screen push when one needs you. It is a read-only consumer of
`gtmux serve` whose look mirrors the menu-bar app, so all three surfaces read as
one product.

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
event, show an in-app banner on a foreground `alert`, and reflect connection
state. `/api/agents` is the only data source.

#### Scenario: Live update

- **WHEN** an agent's status changes on the Mac
- **THEN** the Radar updates via the SSE-triggered refetch

### Requirement: Detail + focus

The system SHALL show a selected pane's current screen (read-only) and offer a
"focus on Mac" action that calls `/api/focus`.

#### Scenario: Read a pane

- **WHEN** the user opens an agent's Detail
- **THEN** the pane's current screen text is shown and kept fresh

### Requirement: Push registration + tap deep-link

The system SHALL, when paired and push is enabled, request notification
permission, register the APNs device token to the Mac, and deep-link a tapped
notification to that agent's Detail (including cold start).

#### Scenario: Tap a push

- **WHEN** the user taps a delivered push carrying a `pane`
- **THEN** the app opens to that agent's Detail

### Requirement: Read-only scope

The system SHALL NOT send keystrokes or run commands (no terminal input); that is
a later, separately-gated phase.

#### Scenario: No write surface

- **WHEN** using any current screen
- **THEN** no action writes to a terminal or runs a command
