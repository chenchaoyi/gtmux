## MODIFIED Requirements

### Requirement: Instructions have durable and honest delivery receipts
The workspace SHALL save scoped user instructions before attempting delivery, protect
drafts and waiting prompts, and separate delivery from execution. Duplicate IDs with
identical content SHALL converge; mismatching reuse SHALL fail. Uncertain attempts
SHALL NOT automatically replay. Failed reads or writes SHALL be visible.

The workspace MAY deliver to a working HQ when the verified input route accepts it and the composer is empty. Queue acceptance SHALL be distinct from execution. Explicit acknowledged retry of uncertain delivery SHALL create a new transport generation; ordinary drains SHALL retain the same idempotency identity.

#### Scenario: Busy HQ and process failure
- **WHEN** HQ is working with an empty composer and a supported guarded input route
- **THEN** the instruction MAY be accepted into its queue without claiming execution
- **WHEN** HQ has an input draft or waiting prompt
- **THEN** the instruction remains pending without overwriting or submitting the draft
- **WHEN** a sender disappears after input may have occurred
- **THEN** the receipt is uncertain and is not blindly submitted again

#### Scenario: Automatic wake competes with an instruction
- **WHEN** another gtmux HQ input channel owns the input lease
- **THEN** the competing instruction or automatic wake remains queued without typing
- **AND** a later drain can deliver it after the lease is released
