# Tasks — usage-watch

## 1. Extraction (`internal/usage`)

- [x] 1.1 Parse the transcript tail for usage rows: cumulative in/out, live
      context footprint (last assistant input+cache_read+cache_creation),
      ctx fraction vs the model window, sliding-window rate (10 min). Ride the
      incremental loader; Claude-first; empty fields when absent. Unit tests on
      jsonl fixtures.
- [x] 1.2 Thresholds: load ~/.config/gtmux/usage.json (per-agent-type layers:
      ctxWarn / sessionTokWarn / typeRatePerMinWarn) with defaults; pure
      evaluate(current, rate, horizon) → first breached/projected layer + ETA.
      Table tests.

## 2. Surfacing

- [x] 2.1 `gtmux usage [--json]`: per-session rows + per-agent-type rollup.
- [x] 2.2 digest rows gain tok/ctx/rate/usage_warn (additive omitempty).
- [x] 2.3 `GET /api/usage` (bearer-gated, additive) + handler test.
- [x] 2.4 Radar: amber usage modifier on the row (like errored/bg) in
      agents --json (`usage_warn` additive field only in P1 — surfaces render
      in a follow-up if needed beyond the JSON).

## 3. Warnings

- [x] 3.1 Hook-side (or serve-tick?) evaluation → the HQ nudge line
      `[gtmux] usage·warn <loc> — <detail>`; dedupe per session+layer via a
      state marker; hqNudge config honored. DECIDE in design: hooks fire only on
      lifecycle events (a long silent turn burns tokens without hooks) — the
      serve events tick (1.5s) may be the better evaluator; pick and document.
- [ ] 3.2 HQ playbook: append a usage-policy section for NEW seeds; release
      note for existing homes (never overwritten).

## 4. Docs + hygiene

- [x] 4.1 README(.zh) + docs/cli.md: `gtmux usage` + thresholds config.
- [x] 4.2 CLAUDE.md contracts note (/api/usage, usage_warn field).
- [ ] 4.3 On merge: sync-specs + archive.

## 5. Gate

- [x] 5.1 make check green; cgo-free build green.
- [x] 5.2 Dogfood: real fleet shows totals/rates; force a low threshold in
      usage.json → the warn fires once into HQ.

## 6. Deferred (P2)

- [ ] 6.1 Subscription-window awareness (Max 5h/weekly) given a reliable local
      source; $ estimation.
- [ ] 6.2 Codex usage parsing; menu-bar/mobile usage badges.
