## ADDED Requirements

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
