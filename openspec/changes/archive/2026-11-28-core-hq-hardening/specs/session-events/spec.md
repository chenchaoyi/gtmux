## ADDED Requirements

### Requirement: Serialized sequence append and tail-first delta reads

`events.Append` SHALL hold an exclusive advisory lock across sequence assignment, size-cap rotation, and `O_APPEND` write so concurrent writers commit strictly ordered sequences without double-rotating `events.1.jsonl`. Positive-cursor `ReadSince` and positive-window `Read` SHALL scan backwards from the tail of `events.jsonl` and read `events.1.jsonl` only when `events.jsonl` does not cover the requested boundary.

#### Scenario: Concurrent appends across rotation preserve sequence order

- **WHEN** multiple writers append events concurrently as `events.jsonl` crosses its rotation cap
- **THEN** `events.1.jsonl` rotates once per cap crossing and `ReadSince` observes contiguous, ordered sequence numbers without false gaps

#### Scenario: Recent delta read avoids scanning rotated log

- **WHEN** `ReadSince(cursor)` or `Read(sinceSecs, now)` is satisfied within the tail of `events.jsonl`
- **THEN** it returns oldest-first matching records and accurate gap flags without reading `events.1.jsonl`
