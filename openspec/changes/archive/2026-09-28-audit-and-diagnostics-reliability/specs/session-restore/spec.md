## ADDED Requirements

### Requirement: Unchanged resurrect snapshots do not cause a save loop

The restore backstop SHALL distinguish a successfully checked layout from a snapshot
whose content changed. An unchanged `last` pointer after a successful `save.sh` run
SHALL NOT cause another attempt on each serve tick. A failed attempt SHALL be retried
after a shorter bounded interval, and a newer snapshot SHALL still suppress a
redundant backstop save.

#### Scenario: An unchanged layout

- **WHEN** `save.sh` succeeds but leaves `last` untouched because the layout is identical
- **THEN** the next serve tick does not invoke `save.sh` again
- **AND** the backstop checks again after its normal interval

#### Scenario: A failed save

- **WHEN** `save.sh` fails while the snapshot remains stale
- **THEN** the backstop retries after a bounded failure interval, not every tick
