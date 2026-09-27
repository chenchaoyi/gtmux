## ADDED Requirements

### Requirement: Mobile views SHALL cancel pending callbacks on unmount

The mobile app SHALL cancel pending copy-feedback timers and chat animation frames
when their views unmount.

#### Scenario: A user leaves after copying a share value or while chat frames are queued

- **WHEN** the share sheet or chat view unmounts
- **THEN** its pending feedback timer or animation frames SHALL be cancelled
