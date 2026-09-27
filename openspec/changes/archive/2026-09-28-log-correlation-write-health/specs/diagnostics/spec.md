## ADDED Requirements

### Requirement: Failed diagnostic writes are visible

A failed diagnostic append SHALL leave the primary action running, SHALL warn on
stderr with the store and filesystem error but no entry body, and SHALL limit
repeated warnings from one store. `gtmux doctor` SHALL probe whether the active
diagnostic file and event journal can be opened for append and whether their
directories can create files. It SHALL also probe an existing HQ knowledge ledger.

#### Scenario: Log path is unavailable

- **WHEN** a diagnostic entry cannot be appended
- **THEN** the action continues and stderr reports that diagnostics could not be written

#### Scenario: A store has become unwritable

- **WHEN** doctor runs while an event file cannot be opened for append
- **THEN** its recording row reports the failure

### Requirement: Audited actions have a correlation key

An audited action with both an event and a diagnostic receipt SHALL carry the
same `op_id` in both records. The key SHALL contain no user content.

#### Scenario: Concurrent sends

- **WHEN** two sends settle on the same pane within one second
- **THEN** each event can be paired with its own diagnostic action by `op_id`
