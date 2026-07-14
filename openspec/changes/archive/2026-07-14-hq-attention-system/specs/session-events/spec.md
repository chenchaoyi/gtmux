## ADDED Requirements

### Requirement: Events carry a monotonic sequence

Every event record SHALL carry an additive, strictly increasing `seq` field assigned at
the single append path from a persistent counter, so consumers have a total order and a
durable cursor position independent of file byte offsets (which rotation invalidates).
Concurrent appends SHALL each receive a distinct, increasing sequence (the counter is
serialized with an advisory file lock, cgo-free). The field is additive to the stable
event contract — a legacy record without it SHALL still read (treated as sequence-unknown,
ordered by timestamp) and MUST NOT fail the reader. Assigning the sequence SHALL keep the
append best-effort and fire-and-forget so a busy or absent consumer never blocks the hook.

#### Scenario: Each event gets an increasing sequence

- **WHEN** two events are appended, even concurrently by separate hook processes
- **THEN** each record carries a distinct `seq` and the later-assigned one is greater

#### Scenario: The sequence survives rotation

- **WHEN** the journal rotates and events keep being appended to the fresh file
- **THEN** the `seq` continues increasing across the rotation boundary (it is not reset)

#### Scenario: A legacy record without seq still reads

- **WHEN** a record written before this field is read back
- **THEN** it is read without error and ordered by timestamp

### Requirement: Resume-from-cursor subscription

The events reader SHALL support resuming from a consumer's cursor (a last-consumed `seq`):
a consumer that reconnects SHALL be able to replay EXACTLY the events with `seq` greater
than its cursor, across both the active and rotated generations, ordered by sequence, so a
crashed or restarted consumer loses no events and never re-emits a consumed one. The reader
SHALL expose enough to DETECT a gap (a missing sequence between the cursor and the next
available event) so a consumer can trigger reconciliation rather than proceed blind.

#### Scenario: Reconnect replays only the un-consumed tail

- **WHEN** a consumer with cursor N reconnects and the journal has advanced past N
- **THEN** it receives every event with `seq > N` once, in sequence order, spanning a
  rotation if one occurred

#### Scenario: A missing sequence is detectable

- **WHEN** the next available event's `seq` is not exactly one past what the consumer
  expected (a hole)
- **THEN** the reader surfaces that a gap exists so the consumer can reconcile
