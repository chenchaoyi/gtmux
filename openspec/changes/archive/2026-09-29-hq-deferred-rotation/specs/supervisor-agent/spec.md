## MODIFIED Requirements

### Requirement: HQ's seeded charter teaches a complete, unprompted self-rotation ritual

After updating the board and knowledge base and recording the handoff, HQ SHALL run
`gtmux hq --rotate` to queue the reset, then end its turn. It SHALL NOT interpret the
queue receipt as completion. The successor session SHALL re-read the board before
acting. A repeated `self-rotate` wake without a changed session ID means the prior
rotation did not take.

#### Scenario: HQ queues its own rotation

- **WHEN** HQ receives a `self-rotate` wake during an active turn
- **THEN** it updates its records, queues rotation, and ends the turn so the resident
  service can deliver the reset after the task is complete
