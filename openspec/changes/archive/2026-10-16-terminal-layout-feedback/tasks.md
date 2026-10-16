## Implementation
- [x] Add a cancellable reflow lifecycle with loading before the expensive grid rebuild.
- [x] Preserve logical line/cell, partial-row offset and follow intent across width changes.
- [x] Cover same-height content, intermediate scroll callbacks, rapid requests, selection and unmount cleanup.
- [x] Run relevant checks; update bilingual docs and archive the synced spec.

Validation: final mobile gate passed (158 suites, 1700 tests; 15 skipped). Cell-anchor and intermediate-clamping mutations each failed the intended regression tests. Same-height layouts, rapid reversals before/after commit, pending polls, selection deferral and cleanup are covered. Native device redraw timing and actual VoiceOver remain unverified; no device build/install in this task.
