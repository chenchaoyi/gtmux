# mobile-app (delta)

## ADDED Requirements

### Requirement: The knowledge sheet follows the app's language for content too

The knowledge sheet SHALL show each entry's half matching the app's language, fall back to
the other half with a small language tag, and match find against both halves.

#### Scenario: A Chinese base read on an English phone

- **WHEN** the phone's language is English and an entry has no English half
- **THEN** the entry shows its Chinese with a `zh` tag, and nothing on the sheet is hidden
