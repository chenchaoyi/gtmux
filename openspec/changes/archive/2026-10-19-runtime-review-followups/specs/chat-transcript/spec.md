## ADDED Requirements

### Requirement: Bound reusable legacy transcript retention

The legacy Claude/Codex loader SHALL retain at most 16 parsed files under a conservative 32 MiB cache accounting budget and evict entries unused for 24 hours on subsequent cache access. Eviction SHALL discard reusable parses only, preserving source history, returned content, file-identity replacement checks and incremental append behavior. Oversized histories MAY be returned without being cached. This budget SHALL NOT be represented as a process RSS limit or a bound for separate native readers.

#### Scenario: A reader visits many large histories

- **WHEN** parsed histories exceed the retained byte or entry budget
- **THEN** least-recently-used entries are evicted and a later read reconstructs the same public history from its source

#### Scenario: A cached file is replaced or expires

- **WHEN** the file identity changes or its idle time reaches 24 hours
- **THEN** the next read reloads the file instead of returning the old parse
