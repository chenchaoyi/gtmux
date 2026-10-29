## MODIFIED Requirements

### Requirement: Exact retirement of provisional Codex native observations
After successful submitted-task verification binds a Codex thread to a live tmux
incarnation, the system SHALL retire only an unchanged minimal startup observation:
Codex, matching home/thread/cwd, PID-less idle, no command/terminal/launch-host/turn/outcome, and an
explicit SessionStart or conservative legacy startup shape within the binding window.
A verified live frontend connection MAY additionally retire an unchanged idle SessionStart from its exact shared backend PID/birth, matching home/thread/cwd and carrying no command/terminal/launch-host/turn/outcome. This exact connection proof does not require the legacy preparation window. Actual turn activity, a different backend or a newer native generation SHALL survive.
An asynchronous explicit start MAY arrive after preparation but not confirmation;
a legacy record SHALL predate preparation. Real native turns/processes SHALL NOT be
retired merely because they share a thread ID, directory or title.

Native writes, retirement and pruning SHALL use atomic publication and a bounded
cross-process store lock. Each new write SHALL have a unique generation, even in the
same second. Retirement SHALL compare the exact generation and publish a private
receipt before removal. Failed receipt writes SHALL preserve the retry witness;
committed pane bindings SHALL still route hooks to the verified pane. Receipt-backed
interrupted removals SHALL reconcile without suppressing later activity. Completed
historical receipts SHALL be bounded to 256; observations still on disk SHALL retain
recovery receipts. Unknown old cases without exact proof SHALL keep existing grace.

#### Scenario: Startup binding then resource closure
- **WHEN** a minimal pane-less start is exactly bound to its worker and the worker later closes
- **THEN** the startup observation SHALL not resurface in the native list
- **AND** Codex history SHALL remain without archive/delete

#### Scenario: Same-second independent activity wins
- **WHEN** another process writes a new generation before retirement or pruning
- **THEN** the newer record SHALL survive even if fields/timestamp match the old record

#### Scenario: Receipt fails after pane binding
- **WHEN** pane identity commits but the retirement receipt cannot be written
- **THEN** the hook SHALL use the verified pane, report failure and retain its retry witness
- **AND** it SHALL NOT fabricate a native turn by falling back to a missing pane

#### Scenario: Partial receipt recovery
- **WHEN** an exact retired snapshot remains after receipt publication
- **THEN** reconciliation SHALL remove only that snapshot, preserving fresh activity

#### Scenario: Old case lacks exact proof
- **WHEN** an old provisional-looking record has no verified binding or matching receipt
- **THEN** normal liveness/grace SHALL apply; saved history alone SHALL not authorize deletion

#### Scenario: Shared backend remains alive after frontend exit
- **WHEN** an exact frontend receipt retired only that backend's unchanged minimal startup observation
- **THEN** closing the frontend does not resurrect the startup as a live external session
- **AND** later independent native activity remains visible
