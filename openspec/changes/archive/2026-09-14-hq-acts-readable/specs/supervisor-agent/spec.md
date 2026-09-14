## ADDED Requirements

### Requirement: The supervision's acts are read over a week

An acts-only read of the journal (`GET /api/hq/events?acts=1`) SHALL look back seven
days, so a weekly tally built on it counts a week; the unfiltered feed SHALL keep its
one-day window.

#### Scenario: A dispatch three days ago is still in the acts feed

- **WHEN** a `gtmux:audit:send` record is three days old
- **THEN** the acts-only read returns it, and the plain feed does not
