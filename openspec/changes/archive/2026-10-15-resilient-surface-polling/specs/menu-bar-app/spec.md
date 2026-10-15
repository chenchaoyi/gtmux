## ADDED Requirements

### Requirement: Bounded periodic CLI reads

Periodic menu-bar CLI reads SHALL have execution and output bounds and at most one operation per polling lane. Failures SHALL retain the last valid snapshot. Gates SHALL be released after success, launch failure, timeout and invalid output.

#### Scenario: A radar subprocess fails or stalls

- **WHEN** a periodic radar query fails or exceeds its deadline
- **THEN** the last valid fleet remains visible and a subsequent poll can run
