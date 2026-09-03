# remote-access (delta)

## ADDED Requirements

### Requirement: The HTTP contract carries the knowledge base, owner-only

`GET /api/hq/knowledge` SHALL serve the knowledge index, `GET /api/hq/knowledge/entry?id=`
one entry with its body, and `POST /api/hq/knowledge/act` the two remote mutations
(`land`, `retire`). All three SHALL be OWNER scope and SHALL refuse a guest `403`, the
same rule `/api/hq/board` and `/api/hq/events` follow, and for the same reason: the base is
the supervisor's private assessment, not part of a scoped share.

A serve running on a machine with no HQ home SHALL answer the reads `200` with empty
collections rather than an error — "no knowledge base" is an ordinary state a client
renders, not a failure it must handle.

#### Scenario: A mutation names an entry that is not live

- **WHEN** `POST /api/hq/knowledge/act` names an unknown or already-retired id
- **THEN** the response is `400` with a message naming the id, and nothing changes

#### Scenario: A malformed act is refused before it reaches the ledger

- **WHEN** the request carries an unknown verb, or `land` with no ref, or `retire` with no
  reason
- **THEN** the response is `400` and the ledger is untouched
