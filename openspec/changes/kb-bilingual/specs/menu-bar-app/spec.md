# menu-bar-app (delta)

## ADDED Requirements

### Requirement: The knowledge window follows the app's language for content too

The knowledge window SHALL show each entry's half matching the app's language setting,
fall back to the other half with a small language tag, and match search against both.

#### Scenario: Switching the app to English

- **WHEN** the language setting changes to English while the knowledge window is open
- **THEN** entries with an English half re-render in English without a refetch, the rest
  show Chinese with a `zh` tag
