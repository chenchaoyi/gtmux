# mobile-app (delta)

## ADDED Requirements

### Requirement: The phone's knowledge sheet shows the axes and carries an entry

The knowledge sheet SHALL show kind, provenance and audience, group the pool by
neighbourhood, and offer the same carry / feedback / withdraw acts through
`POST /api/hq/knowledge/act`, owner-only.

#### Scenario: A guest opens the sheet

- **WHEN** a guest token reads `/api/hq/knowledge`
- **THEN** it is refused, as every `/api/hq/*` surface is
