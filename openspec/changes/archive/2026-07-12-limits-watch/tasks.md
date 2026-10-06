# Tasks — limits-watch

> **Current context (2026-10-06):** This is the original proposal and completion
> record. Codex's local rollout reader and the mobile usage view have since shipped;
> the P2 list below is not their current delivery status. The command route remains
> configurable, and an empty command leaves cached results and local Codex reads
> available. See [current CLI behavior](../../../../docs/cli.md#gtmux-limits-real-subscription-window-remaining),
> [current requirements](../../../specs/usage-watch/spec.md),
> [the command/cache implementation](../../../../internal/limits/limits.go), and
> [the Codex reader](../../../../internal/limits/codex.go).
> The original captured command output and dogfood checkbox are historical evidence,
> not verification of headless command support or token cost in every current agent
> version. The original text and checkboxes are retained below.

- [x] 1.1 `internal/limits`: run the configurable command (default `claude -p
      "/usage"`; env-prefix supported), PURE parser of the window lines →
      [{label,pctUsed,resetAt}]. Table tests over captured fixtures (session +
      2 weekly lines, and a garbled line).
- [x] 1.2 Cache to state/limits.json with TTL (default 15m; 5m when any window ≥ limitsNearPct=70); refresh-if-stale;
      `--refresh` forces; `limitsCommand:""` disables. Never per-call spawn.
- [x] 2.1 `gtmux limits [--json|--refresh]` + a `limits` block on `gtmux usage`
      and the usage report (→ GET /api/usage).
- [x] 2.2 Warn: per-window threshold (limitsWarnPct, default 85) → amber marker
      + HQ `[gtmux] limits·warn …` nudge, deduped per window.
- [x] 3.1 HQ playbook: status reports include the subscription-window line.
- [x] 3.2 Docs (cli.md/README) + CLAUDE.md contract note; sync-specs + archive.
- [x] 4.1 make check green; dogfood: real windows shown; forced-low threshold warns HQ.
- [ ] 5.1 (P2 deferred) pace projection; Codex/other plans; menu-bar/mobile pill.
