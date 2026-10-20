## Implementation

- [x] Add scoped incremental DSH token and daily accounting with native replacement/retry semantics.
- [x] Add native account observation and sanitized balance cache with switch/failure boundaries.
- [x] Expose relevant usage sources and per-source freshness; keep recorded subscription evidence distinct.
- [x] Update shared CLI/mobile/menu-bar quota and balance presentation.
- [x] Add meaningful source, accounting, freshness and UI regressions.
- [x] Update API, per-agent integration and bilingual design/user documentation.
- [x] Run required checks and synchronize/archive specifications.

## Acceptance

Author core race/static checks, full mobile tests and Swift compilation/targeted model tests passed. Real-account balance, physical devices, iPad, Android, VoiceOver and independent review are not claimed. Unsupported API-key/custom-provider paths remain explicit. The embedded Web has no quota page; its additive owner-only API is unchanged in authority.
