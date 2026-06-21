# Menu-Bar App Specification

## Purpose

An always-visible macOS menu-bar app that shows, at a glance, the most-urgent
agent state and a popover list grouped by who needs you. It is a pure consumer of
the CLI (polls `gtmux agents --json`, shells out to `gtmux focus`) and the click
target for notifications.

## Requirements

### Requirement: Ambient status item

The system SHALL render an `NSStatusItem` whose glyph encodes the most-urgent
state by color + shape (waiting → working → idle → calm), with a count badge of
the most-urgent actionable count.

#### Scenario: Most-urgent wins

- **WHEN** at least one agent is waiting
- **THEN** the status item shows the waiting glyph (red square + pause) with the
  waiting count

### Requirement: Grouped popover

The system SHALL show a popover listing agents grouped in fixed order
waiting → working → idle → running, only non-empty sections, each row carrying
the agent avatar + status badge + session/task, with the waiting section
emphasized.

#### Scenario: Jump from a row

- **WHEN** a row is clicked (or Enter / ⌘1–9)
- **THEN** the app runs `gtmux focus <pane>` and lands on that agent

### Requirement: Pure CLI consumer

The system SHALL source all data from `gtmux agents --json` and SHALL NOT
duplicate detection logic; gtmux-core stays the single data source.

#### Scenario: Poll for updates

- **WHEN** the refresh timer fires
- **THEN** the app re-runs `gtmux agents --json` and repaints

### Requirement: Notification click target

The system SHALL be the notification target (`com.gtmux.menubar`): it drains the
notify queue, posts native banners, and on click jumps to the last-finished
agent.

#### Scenario: Click a banner

- **WHEN** the user clicks a delivered notification
- **THEN** the app activates and runs `gtmux focus --last`
