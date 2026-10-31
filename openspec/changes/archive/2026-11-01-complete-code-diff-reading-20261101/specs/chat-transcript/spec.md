## ADDED Requirements

### Requirement: Complete selected tool input on reader intent
The server SHALL retain bounded tool previews for polling and provide an optional original-call reference for a complete selected input read. The authenticated read SHALL independently verify session/home, source identity and position, call ordinal and content hash. It SHALL exclude tool results, unrelated arguments and context, deny guests and fail explicitly for missing, rewritten, incomplete or oversized records.

#### Scenario: A later call exhausts a turn preview
- **WHEN** a visible call has no remaining preview budget and the reader opens its details
- **THEN** a compatible owner/full-paired reader can retrieve its complete original selected input without widening regular transcript polls
- **AND** copying cannot silently use a partial preview as complete source
