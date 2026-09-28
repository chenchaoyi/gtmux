## ADDED Requirements

### Requirement: Phone knowledge actions and diagnostics use actionable language

The phone SHALL use the same Chinese name for `retire` as the Mac and describe
the reason and recorded destination of knowledge actions plainly. Pairing and
diagnostic messages SHALL tell the user what happened and the next useful step
without exposing internal token exchange details where they are not needed.

#### Scenario: A pairing code yields no credential

- **WHEN** enrollment cannot complete after a code is scanned
- **THEN** the phone asks the user to refresh the pairing code on the Mac and
  scan again
