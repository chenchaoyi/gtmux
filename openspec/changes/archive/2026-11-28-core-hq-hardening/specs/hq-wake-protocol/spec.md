## ADDED Requirements

### Requirement: Bounded terminal wake evidence and draft-aware retry holds

The HQ wake queue SHALL bound retained terminal files (`.refused`, `.native-uncertain`, `.legacy-uncertain`) by archiving aged or excess entries into a size-rotated private JSONL archive and retiring their `nativeIntent` and `.pin` files. Aged colliding claim slots SHALL archive the older retained file before settling. Native refusals for `InputDraftPresent` or `InputQuestion` SHALL requeue with bounded backoff instead of exhausting into `.refused`.

#### Scenario: Excess terminal wake evidence is archived and unpinned

- **WHEN** retained terminal wake files exceed the retention count or age bound
- **THEN** older entries and uncertain `nativeIntent` records are appended to the bounded terminal archive and their `.pin` and batch files are removed

#### Scenario: Aged colliding slot reconciles cleanly

- **WHEN** recovery encounters a valid `.sending` claim whose target terminal path holds a distinct aged regular file
- **THEN** the older retained file is archived with both digests, the `.sending` claim settles cleanly, and clean recovery can clear `claim-conflict`

#### Scenario: User draft or question holds wake without permanent exhaustion

- **WHEN** native wake delivery is refused with `InputDraftPresent` or `InputQuestion` across more than three retry ticks
- **THEN** the wake remains queued under bounded backoff and delivers once the composer clears

### Requirement: Atomic consumption watermark and outcome tally

HQ's consumption watermark (`consumed-seq`) and tick outcome tally (`tally.log`) SHALL prevent torn reads and lost concurrent updates: `Consumed` SHALL return unset (`-1`) only when the marker file does not exist and `0` when empty or malformed, `Consume` SHALL advance monotonically under an advisory file lock, and `ConsumeTick` SHALL atomically rename `tally.log` under a lock before reading and removing it.

#### Scenario: Concurrent outcome appends survive tick consumption

- **WHEN** a hook appends an outcome to `tally.log` concurrently with `ConsumeTick`
- **THEN** `ConsumeTick` renames and consumes only the claimed batch without deleting outcomes appended afterward
