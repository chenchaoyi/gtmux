# usage-widgets Specification

## Purpose
Display trusted provider capacity or local output records in bounded, credential-free system widgets.

## Requirements

### Requirement: Provider usage at a glance

The system SHALL offer iOS/iPadOS 17+ Home Screen and Lock Screen usage widgets using the existing provider usage report. It SHALL preserve quota and balance units, identify the source server, expose the reading time, and distinguish unavailable, historical and expired data from current readings. It SHALL NOT infer a subscription or reset an expired quota to zero.

#### Scenario: Recorded window expires

- **WHEN** a widget reaches the recorded reset time without a new report
- **THEN** it shows that an updated reading is needed instead of presenting the old percentage as current capacity

### Requirement: Credential-free widget snapshots

The widget extension SHALL read only a bounded typed snapshot from its shared container, without pairing credentials, host URLs, conversation content or provider account identifiers. Refresh work SHALL remain in the parent application using the current owner connection, enforce cancellation and response limits, reject redirects, and discard results from a previous selected server. Disconnect, guest selection, unpair and authorization rejection SHALL clear shared usage state.

#### Scenario: Server changes during a refresh

- **WHEN** a previous server response arrives after selection changes
- **THEN** it cannot replace the new server snapshot or restore removed data

### Requirement: Native platform delivery is explicit

This delivery SHALL share the snapshot model and SwiftUI views with macOS 14+, while offering native usage widgets on iOS/iPadOS only. Native macOS widget delivery SHALL require a separately compiled sandboxed extension and verified shared-container/signing arrangement before it is advertised. iPhone widget Continuity SHALL be described separately. Refresh timing SHALL remain subject to WidgetKit and background execution budgets.

#### Scenario: Background refresh is delayed

- **WHEN** the platform postpones refresh work
- **THEN** the widget identifies the old reading and offers an app entry point without claiming a live connection

### Requirement: Saved provider choice remains explicit

Widget configuration SHALL offer Automatic and the distinct providers present in the display snapshot, with stable nonempty identifiers and readable names. Automatic SHALL prioritize current readings over older readings, then the most constrained quota. Selecting an unavailable provider SHALL show unavailable data rather than choosing another provider.

#### Scenario: Selection persists beyond the editor

- **WHEN** a user selects DeepSeek in the system widget editor
- **THEN** the rendered widget uses the saved DeepSeek choice and shows its available recorded output or actual capacity reading with the correct units

### Requirement: Recorded output is a distinct fallback

Usage widgets SHALL display an agent's recorded daily and seven-day output tokens
when the report has no quota or balance reading for that agent. They SHALL identify
the data as local records, retain the source day and scan time, and never convert
tokens into money, remaining quota or an account-wide billing claim. Absent history
SHALL remain unavailable, while an explicitly reported zero SHALL remain zero.

#### Scenario: DSH has output history but no balance

- **WHEN** the report contains DSH output history and no DSH capacity reading
- **THEN** selecting DSH displays recorded output tokens with a local-record label and no currency or quota gauge

#### Scenario: A cached day ends

- **WHEN** the recorded day no longer matches the display's current calendar day
- **THEN** the widget labels the recorded date instead of calling the value today's output

#### Scenario: Capacity data is available

- **WHEN** the same provider has an actual quota or balance reading
- **THEN** that reading retains its original units and takes precedence over the output fallback
