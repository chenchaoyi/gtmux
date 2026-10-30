## ADDED Requirements

### Requirement: Progress-oriented trace reading
Native Trace SHALL open at the newest recorded activity without a Follow latest mode switch. It SHALL group records by their exact conversation and stable turn identity, provide a latest-turn summary, and compact consecutive tool calls without discarding records. Refresh SHALL preserve a reader away from the tail and the selected detail revision. Near-tail readers MAY continue following new activity.

#### Scenario: Inspect history while the agent updates
- **WHEN** a reader scrolls away from the tail or inspects a historical record
- **THEN** appends, streamed text and loaded earlier history do not pull the viewport away
- **AND** a one-shot latest action can return to the newest activity
