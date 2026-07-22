# remote-access — delta

## ADDED Requirements

### Requirement: Supervisor knowledge is served to owner clients

The server SHALL expose the supervisor's situation board and the severity-tagged event
ledger to owner clients, so a remote surface can present the supervisor's own assessment
and the fleet's history rather than only the present instant. The board SHALL be served
read-only, with the time it was last written, and SHALL report its absence as an ordinary
state rather than an error, since a supervisor that has written no board yet is normal.
The event ledger SHALL be filterable by a severity floor and bounded in length, because a
remote client must not have to download the whole log to show recent activity. Both
SHALL be refused to a guest caller: they carry the whole fleet and the supervisor's
private assessment, which are owner surfaces and never part of a shared scope. Both SHALL
remain available when no supervisor is running, reporting empty rather than failing.

#### Scenario: An owner client reads the supervisor's board

- **WHEN** an owner-scoped client requests the situation board
- **THEN** it receives the board's text and the time it was last written

#### Scenario: No board has been written

- **WHEN** the supervisor has never written a situation board
- **THEN** the request succeeds and reports that no board exists, rather than erroring

#### Scenario: Recent activity is bounded and filtered

- **WHEN** an owner-scoped client requests the event ledger at a severity floor
- **THEN** it receives only records at or above that severity, newest first, no more
  than the requested number

#### Scenario: A guest cannot read either

- **WHEN** a guest-scoped caller requests the board or the event ledger
- **THEN** the request is refused, exactly as for the digest and usage surfaces
