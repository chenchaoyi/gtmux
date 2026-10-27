## MODIFIED Requirements

### Requirement: Native platform delivery is explicit

The snapshot model and SwiftUI views SHALL be shared by iOS/iPadOS widgets and
a separately compiled native macOS 14+ sandboxed WidgetKit extension. The Mac app
SHALL embed the extension and its resource bundle and sign it before the parent.
Both bundles SHALL contain resolvable configuration metadata and matching release
and build versions. The initial configuration SHALL permit Automatic selection.
Native Mac delivery SHALL use a separate signer-derived team-prefixed App Group,
not the iOS container or shared credentials. The CLI SHALL have no group grant;
the extension SHALL have no network or automation grant. iPhone widget Continuity
SHALL be described separately. Refresh timing SHALL remain subject to WidgetKit
and background execution budgets.

#### Scenario: Background refresh is delayed

- **WHEN** the platform postpones refresh work
- **THEN** the widget identifies the old reading and offers an app entry point without claiming a live connection


## ADDED Requirements

### Requirement: Bounded native Mac refresh and handoff

The menu-bar parent SHALL poll the existing usage CLI only when a native usage widget
is configured, at least 60 seconds between starts, with bounded time/output and no
overlap. Timeline reload requests SHALL be at least 300 seconds apart. Failure SHALL
retain a dated reading rather than invent a current value. The sandboxed extension
SHALL read only the existing typed bounded display projection.

#### Scenario: No widget or overlapping refresh

- **WHEN** no native widget is configured or a poll is already running
- **THEN** the parent SHALL NOT launch a new usage poll

#### Scenario: Widget opens usage

- **WHEN** the app receives the exact usage shortcut without credentials, query,
  fragment, port or alternate path
- **THEN** it SHALL open the existing usage page without starting HQ
- **AND** generic reopen focus SHALL NOT jump to a terminal instead

#### Scenario: Unsigned local build

- **WHEN** no Developer ID team can be derived
- **THEN** the extension may compile but SHALL NOT claim group sharing or receive the
  iOS container as a fallback
