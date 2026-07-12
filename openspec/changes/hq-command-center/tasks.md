# Tasks — HQ command center (mobile)

- [ ] 1.1 `client.digest()` (+ types) over GET /api/digest; reuse usage/limits.
- [ ] 1.2 RadarScreen: a supervisor card tap routes to `HQ` (new screen), not `Detail`.
- [ ] 2.1 HQScreen status strip: fleet counts (from digest) + week/plan % (from usage).
- [ ] 2.2 DigestBoard: needs-you→working→idle rows (badge/loc/agent/goal/ask);
      tap = select (binds command context); long-press = navigate to Detail.
- [ ] 2.3 Command console: ChatView(HQ transcript) + command bar (free text → HQ)
      with quick chips (现状/谁在等我/用量额度; per-target continue/inspect/reply).
- [ ] 3.1 Bilingual; light+dark; matches MOBILE.md; add the HQ screen to MOBILE.md.
- [ ] 3.2 jest: supervisor routes to HQ screen; digest board grouping/selection;
      quick-chip → send payload. sync-specs + archive on merge.
- [ ] 4.1 npm run check green; sim dogfood: fleet board reflects live digest,
      quick commands reach HQ, long-press jumps.
- [ ] 5.1 (P2 deferred) menu-bar HQ command popover; voice input; HQ one-tap actions.
