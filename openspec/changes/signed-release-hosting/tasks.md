## Implementation

- [ ] Define and test a bounded signed manifest shared by native/bootstrap/publisher.
- [ ] Add official mirrors without weakening checksum, source or redirect boundaries.
- [ ] Publish unchanged GitHub assets only after both release components succeed.
- [ ] Build and test isolated R2 download/publication service, conditional/range reads and immutable promotion.
- [ ] Update installer, paired docs, website and affected Store information consistently.
- [ ] Validate failure paths, credential separation and deployment gates; record live delivery acceptance separately.

## Acceptance

Do not equate code/CI with deployed DNS, uploaded files, successful installation or
account-level billing headroom. Report each separately. Keep private phone delivery
and existing site access policies unchanged.

Local core/mac complete builds encountered ENOSPC; focused Go tests, synthetic signing/bootstrap/publisher tests and Worker tests pass, with full hosted gates still required. No independent review, physical-device or live download/DNS acceptance is claimed by these author checks.
