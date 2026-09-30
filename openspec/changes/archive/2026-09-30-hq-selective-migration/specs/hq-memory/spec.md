## ADDED Requirements

### Requirement: HQ records expose restore and selective migration
The menu bar SHALL expose full backup restore and migration from another Mac separately.
Migration SHALL preview and stage only selected long-term knowledge and personal requirements,
with tools as non-executable attachments. Board, session/control state and credentials SHALL
NOT migrate. Selection SHALL start empty, and sensitive families require a separate choice.

#### Scenario: Moving to a new Mac
- **WHEN** an owner stages selected knowledge from an HQ archive
- **THEN** the current HQ is unchanged, old board and control state are excluded, and
  knowledge is available for review before becoming active

### Requirement: Archive writes and migration applications preserve current data
Archives SHALL be fully validated with bounded decompression before live writes. Unsafe paths,
links, special files, duplicate names and bad encryption/checksums SHALL fail without changing
the destination. Selective migration SHALL refuse unsupported ledger records. Restores SHALL retain the displaced home.
Knowledge applications SHALL retain source lineage, skip identical history, refuse conflicting
IDs, atomically publish the complete ledger and keep a private backup. Old audience/promotion
decisions SHALL be reset. Personal requirements SHALL be reviewed and applied separately with
an explicit replacement choice and retained backup. Live apply/restore SHALL refuse a running HQ.

#### Scenario: A damaged archive fails late
- **WHEN** a valid HQ entry is followed by an unsafe or damaged entry
- **THEN** validation fails before the old HQ directory is moved

#### Scenario: Knowledge applies twice
- **WHEN** the same selected source history is applied again
- **THEN** identical history is skipped and no duplicate entry or promotion is created

### Requirement: Migration applications have explicit decisions and receipts
Knowledge and personal requirements SHALL apply separately. Knowledge SHALL require selected
IDs and explicit review; LOCAL.md replacement SHALL require a fresh destination digest.
Preview SHALL be read-only, staging SHALL be resumable and private, and application receipts
SHALL distinguish pre-commit failure from committed view/sync failure. Startup, restore and
application SHALL share a lock outside the replaceable home. Plain/encrypted exports SHALL
use private unique temporary files.

#### Scenario: The destination changes after personal requirements review
- **WHEN** LOCAL.md no longer matches the reviewed digest
- **THEN** replacement fails and retains the current text

#### Scenario: An import overlaps HQ startup
- **WHEN** another startup, restore or application holds the publication lock
- **THEN** the operation reports contention and does not write live records

#### Scenario: The current ledger is damaged during backup recovery
- **WHEN** an owner previews a valid full backup while the destination ledger is unreadable
- **THEN** restore preview validates the source without depending on the damaged destination
