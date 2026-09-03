# hq-knowledge (delta)

## ADDED Requirements

### Requirement: The commander can act on knowledge remotely, through a second narrower door

`gtmux serve` SHALL expose the knowledge base to an OWNER-authenticated client, and SHALL
accept from it exactly two mutations: `land` (close a pending promotion with a ref) and
`retire` (remove a live entry with a reason). A GUEST SHALL be refused, like every other
`/api/hq/*` surface.

The CLI's cwd-keyed HQ-home gate SHALL remain unchanged. The two doors exist for two
different callers: the cwd gate keeps WORKERS out of the quality gate, while the serve
door is the COMMANDER, who outranks the supervisor and is the only party who can know that
a promotion actually landed somewhere durable. Both doors SHALL journal the same
`gtmux:audit:knowledge` record, so the change history stays one stream regardless of which
door a mutation came through.

`add` and `supersede` SHALL NOT be exposed remotely. They carry prose, and the quality of
the base is the point of having one.

#### Scenario: A guest cannot see or write knowledge

- **WHEN** a client holding a guest token requests any knowledge endpoint
- **THEN** the request is refused `403`, and no entry, title or count is disclosed

#### Scenario: The commander closes a promotion from the phone

- **WHEN** an owner-authenticated client posts `land` for an entry with a pending
  promotion, carrying a ref
- **THEN** the ledger records the landing, the promotion brief is removed, the topic
  render is refreshed, and one `gtmux:audit:knowledge` record is journaled — identically
  to the same verb run from the HQ home

#### Scenario: Landing something that was never promoted is refused

- **WHEN** `land` names an entry with no pending promotion
- **THEN** the request fails with a message saying so, and the ledger is untouched

### Requirement: The knowledge read API separates the index from the entry

The read surface SHALL be two endpoints: an INDEX carrying every live entry's identity,
topic, title, timestamp, promotion state and provenance summary but NOT its body, and a
DETAIL endpoint carrying one entry's body.

The split is a size judgment, not a convenience: a base of a few hundred entries with
bodies is megabytes over a tunnel, while the same index without them is tens of kilobytes.
The index SHALL also report the topic vocabulary with per-topic counts, the pending
promotion count with the age of the oldest, and the pending capture-candidate count — the
three figures that say whether the base is being maintained.

#### Scenario: The index is small enough to poll

- **WHEN** the index is requested for a base of several hundred entries
- **THEN** the response carries every entry's identity and state without bodies

#### Scenario: A base nobody has written to is ordinary

- **WHEN** the index is requested on a machine with no HQ home or an empty ledger
- **THEN** the response is `200` with empty collections and zero counts, not an error
