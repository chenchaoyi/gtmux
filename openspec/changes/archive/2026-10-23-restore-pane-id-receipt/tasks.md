## Implementation

- [x] Read pane IDs from saved/live layout metadata and show only evidence-backed pairs.
- [x] Explain layout differences and actions; replace the backup-age copy in both languages.
- [x] Align paired user docs and the layout-warning spec.
- [x] Verify normal, ambiguous, missing/legacy and narrow-terminal output with synthetic data.
- [x] Run required source/design checks and archive the implemented change.

## Acceptance

Actual reboot, user-session restoration, terminal focus and pane closure are not
performed. Synthetic checks do not establish those runtime results. The receipt
does not map process/conversation identities or repair layout drift automatically.
