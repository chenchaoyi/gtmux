## ADDED Requirements

### Requirement: HQ maintenance distinguishes a trigger from completed work

The distill and self-check sensors SHALL record when a pass is requested. HQ SHALL
explicitly acknowledge completing each pass from its own home. Doctor and the capture
queue SHALL NOT describe a requested but unacknowledged pass as completed. A completion
SHALL have a structured diagnostic action and a journal record identifying the kind
and the request it settles. Repeating a receipt for the same request SHALL be harmless.

#### Scenario: The wake is missed

- **WHEN** a distill request is raised but HQ has not acknowledged completing it
- **THEN** doctor reports the pass as pending rather than recently completed

#### Scenario: HQ finishes a pass

- **WHEN** HQ reviews the event delta and pending captures and acknowledges distill
- **THEN** doctor records completion, and the event and diagnostic trails name the
  same request and its outcome
