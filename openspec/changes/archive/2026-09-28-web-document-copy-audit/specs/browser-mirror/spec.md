## ADDED Requirements

### Requirement: Browser access copy serves owners and guests

The browser access page SHALL explain that its code field accepts either an
owner pairing code or a guest share code. A rejected code SHALL prompt the
reader to check it or get a new one without assuming a share-link sender.
An empty chat view SHALL point to Terminal for the current screen without
misstating how the agent transcript is recorded.

#### Scenario: A Mac owner mistypes a pairing code

- **WHEN** enrollment refuses the code entered on the browser access page
- **THEN** the page gives a usable next step without telling the owner to ask
  another person to resend a share link
