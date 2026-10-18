## ADDED Requirements

### Requirement: Bound hot retained-journal reads

LatestSeq SHALL report the highest retained record sequence, not the last assigned sequence counter. It SHALL incrementally fold appended sequence headers and invalidate removed, replaced, rewritten or truncated generations; unchanged polls SHALL NOT decode history. Recent HQ act reads SHALL scan backward with bounded line/chunk buffers, filter time, severity and current external-conversation consent before applying their result cap, and preserve newest file order across rotation. Out-of-order timestamps SHALL NOT cause an early cutoff that hides eligible records.

#### Scenario: A sequence was assigned but not retained

- **WHEN** the assigned counter is ahead of the journal or a provisional EOF record is invalidated by an append
- **THEN** LatestSeq reflects retained valid records only

#### Scenario: The newest records do not match the current reader

- **WHEN** newer records fail severity or consent filters
- **THEN** the bounded HQ feed continues backward to eligible records before filling its 200-result cap
