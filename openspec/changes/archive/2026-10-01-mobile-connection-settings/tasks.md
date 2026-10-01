## Implementation
- [x] Distinguish failed route reads from valid empty responses; preserve the owner entry and truthful route-page states.
- [x] Guard stale route reads, probes and move receipts across Mac switches; cover failure and retry.
- [x] Redesign server cards, notification notices and More actions; cover owner/guest and accessibility behaviour.
- [x] Simplify the menubar empty state and fold agent-neutral manual instructions; render both languages in light/dark modes.
- [x] Sync English/Chinese design docs, specifications and local release notes.

## Verification
- [x] Run the final mobile, Swift release/test, make and design gates.
- [x] Review phone and iPad simulator layouts. Physical device acceptance remains pending.

After verification, archive this change and submit a PR. Merge only after CI is green.
