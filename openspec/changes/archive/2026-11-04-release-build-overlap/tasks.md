## Implementation

- [x] Separate Mac build and publication with immutable provenance verification.
- [x] Split early iOS preparation from its final publication gate and bounded wait.
- [x] Add synthetic corruption, source drift, failure, pending and rerun regressions.
- [x] Update bilingual runbooks and current capability specifications.

## Acceptance

Run the complete core check, strict specifications, workflow validation and latest
candidate/main platform checks. Save their real results separately. A future signed
release must measure actual overlap and time saved; unit tests do not establish
Apple service latency or physical installation success.
