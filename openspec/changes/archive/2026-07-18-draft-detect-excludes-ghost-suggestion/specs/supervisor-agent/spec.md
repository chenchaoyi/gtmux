# supervisor-agent (delta)

## MODIFIED Requirements

### Requirement: Nudge injection guards a half-typed HQ draft

The system SHALL NOT clobber or auto-submit a half-typed draft in the HQ pane when
injecting a nudge. Before typing, it SHALL read the HQ input box (reusing the
dispatch input-region detector) and, when the draft is non-empty, SHALL NOT type and
SHALL NOT send Enter — the nudge is queued instead. The draft read SHALL be COLOR-aware
and SHALL EXCLUDE the agent's suggested-next-command GHOST text — the dim autosuggestion
rendered faint (SGR 2), which is NOT user input — so a faint ghost suggestion in the HQ
composer does NOT hold a nudge behind a phantom draft; only genuinely half-typed USER
input (normal brightness) SHALL defer delivery. Delivery SHALL occur only when the box
is confirmed empty over TWO reads a short interval apart, and a queued nudge SHALL be
delivered on a later empty box: on the next injection attempt, on HQ's own turn-end
(`Stop`, box reliably empty — coalesced), or on the serve tick. It is an INVARIANT that
no code path sends Enter into a non-empty HQ input box.

#### Scenario: A half-typed draft is never clobbered

- **WHEN** a nudge fires while the HQ input box holds a non-empty draft
- **THEN** nothing is typed and no Enter is sent; the nudge is queued

#### Scenario: A queued nudge is delivered once the box is empty

- **WHEN** the HQ box is confirmed empty over two reads (or HQ finishes a turn)
- **THEN** the queued nudge(s) are delivered, coalesced, exactly once

#### Scenario: A faint ghost suggestion does not hold a nudge

- **WHEN** a nudge fires while the HQ composer shows only the agent's faint
  suggested-next-command ghost text (SGR 2), with no real half-typed input
- **THEN** the ghost text is not read as a draft, so the nudge is delivered rather than
  queued behind a phantom draft
