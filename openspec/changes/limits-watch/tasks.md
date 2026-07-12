# Tasks — limits-watch

- [ ] 1.1 `internal/limits`: run the configurable command (default `claude -p
      "/usage"`; env-prefix supported), PURE parser of the window lines →
      [{label,pctUsed,resetAt}]. Table tests over captured fixtures (session +
      2 weekly lines, and a garbled line).
- [ ] 1.2 Cache to state/limits.json with TTL (default 10m); refresh-if-stale;
      `--refresh` forces; `limitsCommand:""` disables. Never per-call spawn.
- [ ] 2.1 `gtmux limits [--json|--refresh]` + a `limits` block on `gtmux usage`
      and the usage report (→ GET /api/usage).
- [ ] 2.2 Warn: per-window threshold (limitsWarnPct, default 85) → amber marker
      + HQ `[gtmux] limits·warn …` nudge, deduped per window.
- [ ] 3.1 HQ playbook: status reports include the subscription-window line.
- [ ] 3.2 Docs (cli.md/README) + CLAUDE.md contract note; sync-specs + archive.
- [ ] 4.1 make check green; dogfood: real windows shown; forced-low threshold warns HQ.
- [ ] 5.1 (P2 deferred) pace projection; Codex/other plans; menu-bar/mobile pill.
