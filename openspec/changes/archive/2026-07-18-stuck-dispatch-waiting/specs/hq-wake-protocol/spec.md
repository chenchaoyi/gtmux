# hq-wake-protocol — delta

## ADDED Requirements

### Requirement: A stuck-before-running pane wakes as waiting, not done

The wake protocol SHALL NOT fire a `done` wake for a pane that is idle only because it is
blocked at a startup/permission gate or holds a tracked dispatch's undelivered draft, and
SHALL instead fire a `waiting` wake (kind `startup` / `draft`) so a supervisor is nudged
to unblock it. The `waiting` marker driving that wake SHALL be written by the single
writer (the serve slow-tick), never as a side effect of a read-side radar scan.

#### Scenario: Startup-gate idle fires waiting, not done

- **WHEN** the slow-tick finds a tracked dispatch pane blocked at a startup gate (or
  holding its undelivered draft)
- **THEN** it fires a `» gtmux·waiting` signal (not `» gtmux·done`) and records the
  waiting marker so the watchdog escalates the stuck worker

#### Scenario: An incidental Stop does not relabel a stuck pane done

- **WHEN** a `Stop` fires on a pane whose post-Stop screen is a startup gate or a draft
  still holding the payload
- **THEN** no `done` wake is emitted for it
