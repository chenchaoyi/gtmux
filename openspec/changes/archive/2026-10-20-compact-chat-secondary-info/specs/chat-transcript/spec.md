## ADDED Requirements

### Requirement: Profile loading and launch observations have explicit scope

The participant dialog SHALL show an explicit, stable loading state before a profile request settles, without flashing unrecorded or previous-target facts. Failed reads SHALL offer Retry; successful empty results SHALL state that facts were not recorded. A message's historical model, tier and options SHALL remain unchanged when inspecting runtime evidence. A separate on-demand current-profile request MAY show the latest recorded sanitized chain/time only for the same verified source session and agent. Its section SHALL label the current observation scope and SHALL NOT claim live ownership or message-time ancestry. A current header profile SHALL reuse its current response. Stitched older sessions and other senders SHALL NOT borrow another runtime. Missing chains SHALL be explicit.

#### Scenario: First profile read is delayed

- **WHEN** a reader opens an avatar and the request is pending
- **THEN** a labelled placeholder appears at a stable dialog height, without an unrecorded-data notice

#### Scenario: Inspect a message's same-session current chain

- **WHEN** historical facts are visible and the reader opens the current-chain disclosure
- **THEN** only the separately verified same-session chain/time appears, without replacing historical facts with current values

#### Scenario: Changed target or invalid current binding

- **WHEN** the target changes during a read or a current response identifies another session or agent
- **THEN** late or mismatched facts/chain are not displayed or copyable
