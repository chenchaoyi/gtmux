# Verification

- Saved v1.0.111 report: 47 passed, 2 recommended, 0 blocking, 5 informational.
  Recommendations belong to Logs / recent errors and HQ / conversation health.
  The old pending wording counted findings, not unfinished checks.
- CLI tests cover bilingual completion, all classes, full guidance, severity order,
  same-label provenance, and exclusion of passed/informational rows.
- Existing JSON/exit and repair-summary tests pass; no probe or repair behavior changed.
- Swift model tests verify exact original-row targets, unique summary identities,
  severity filtering/order, completed copy, and focused detail expansion.
- Bilingual summary components were visually inspected at 560 pt. Native hosting
  tests verify fixed 560×600 bounds without invoking CLI checks or repairs.
- Full ScrollView ImageRenderer snapshots were discarded: scrolling content and
  native link controls were unsupported. Native UI automation timed out for a
  separate synthetic app. Actual click navigation and VoiceOver are NOT verified.
- Required gates: make check (all Go race tests and 32 publisher tests), full
  Swift suite (399 tests), strict OpenSpec validation, and design conformance.
- No live repair, HQ rotation, installation or release was performed.
