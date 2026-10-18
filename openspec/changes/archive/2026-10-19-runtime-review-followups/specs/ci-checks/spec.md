## ADDED Requirements

### Requirement: Distinguish validator startup failure from invalid specifications

The design gate SHALL require a complete structured verdict from the pinned OpenSpec validator for the current checkout with a nonempty inventory and consistent totals. Invalid specification results SHALL fail as spec errors. Missing tools, invocation/cache failures, timeouts, malformed output, contradictory success and checkout mismatch SHALL fail as tool/setup errors, never success or an asserted malformed-spec verdict without evidence.

#### Scenario: npx cannot start the validator

- **WHEN** the process cannot execute or does not produce a complete current-checkout report
- **THEN** the gate fails with a tool/setup diagnosis

#### Scenario: The validator reports an invalid item

- **WHEN** a complete report contains a failed specification even with process exit zero
- **THEN** the gate fails with the actual invalid item
