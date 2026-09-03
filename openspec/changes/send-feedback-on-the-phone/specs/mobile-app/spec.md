# mobile-app (delta)

## ADDED Requirements

### Requirement: A send that did not land says why, in the core's own words

When `POST /api/send` refuses, the app SHALL show the reason the core gave rather than a
generic failure. The three refusals differ in what the reader should do — someone is
typing in that pane, the pane is gone, the key is not allow-listed — and one bar that
cannot tell them apart sends the reader to the Mac for all three.

An override SHALL exist only for `refused-draft`, and SHALL state what it overrides:
sending anyway submits the other person's unfinished line together with the payload.

#### Scenario: A pane with someone typing in it

- **WHEN** a send is refused because the pane holds unsent text
- **THEN** the app says so in the core's words, and offers to send anyway as a second,
  explicit action

#### Scenario: A pane that is gone

- **WHEN** a send is refused because the pane no longer exists
- **THEN** the app says so, and offers no override
