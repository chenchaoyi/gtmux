## ADDED Requirements

### Requirement: Bounded recovery of a verified connection root
The bridge SHALL retain the latest matching successful non-ephemeral root operation when publication encounters explicitly unavailable terminal proof or a held publication lease. Every retry SHALL use the existing strict identity and owner checks and SHALL NOT replay input or RPC frames.

#### Scenario: Initial publication proof is temporarily unavailable
- **WHEN** the first verified root publication fails without granting authority
- **AND** the same connection remains current and its proof becomes available within the retry budget
- **THEN** the same root can publish and resume its normal heartbeat

#### Scenario: Selection changes during retry
- **WHEN** the connection requests a newer root or a newer connection epoch succeeds
- **THEN** the old pending root cannot publish
- **AND** failure of a successful new root publication cannot refresh the old root as current
