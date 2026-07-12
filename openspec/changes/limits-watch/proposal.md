# limits-watch — real subscription-window remaining (5h / weekly), via `claude -p /usage`

## Why

usage-watch measures LOCAL token burn but not "how much of my plan is left this
week/month" — the user's top question (2026-07-12). Anthropic exposes no local
file or public API for subscription limits, BUT Claude Code's own `/usage`
command reports them authoritatively, and it runs HEADLESS:

```
$ claude -p "/usage"
Current session: 11% used · resets Jul 13 at 1:30am
Current week (all models): 58% used · resets Jul 17 at 10:59pm
Current week (Fable): 88% used · resets Jul 17 at 10:59pm
```

Three clean lines — window label, percent used, reset time — REAL server data
(not estimation), and it's the user's own sanctioned command output, not a
reverse-engineered endpoint. This supersedes the usage-watch P2 "local
estimation of subscription windows": we can show the real remaining %.

## What Changes

- **New capability `limits-watch`** (Claude/Max first):
  - `internal/limits`: run a configurable command (default `claude -p /usage`,
    env-prefixable like `GTMUX_HQ_AGENT` for the home-proxy case) and parse the
    windows into `[{label, pctUsed, resetAt}]`. Runs are **cached** to
    `state/limits.json` with a TTL (default 15 min, shortened to 5 min when a window is near its cap) — it spawns a process, so it
    is NEVER called per `gtmux usage`; a stale cache triggers a refresh at most once per TTL, and `--refresh` forces one.
  - Surfacing: a `limits` block on `gtmux usage` (and `--json` / `GET /api/usage`)
    — `Session 11% · Week 58% (resets Jul 17) · Week/Fable 88%`; a bare
    `gtmux limits [--json|--refresh]`.
  - **Warnings**: a configurable per-window threshold (default: warn when any
    weekly window ≥ 85%, or the reset is far and usage is ahead of pace) → the
    same amber modifier + HQ nudge channel as usage-watch
    (`[gtmux] limits·warn week 88% — resets Jul 17`).
  - HQ playbook: briefings include the subscription-window line (this is the
    "how much room is left" the user asked reports to carry).
- **P2 (deferred)**: pace projection ("at this rate you'll hit the weekly cap ~2
  days before reset"); Codex/other-plan equivalents; menu-bar/mobile limits pill.

## Capabilities

### New Capabilities
- `limits-watch`: headless `/usage` extraction, caching, CLI/API, warn channel.

### Modified Capabilities
- `usage-watch`: the report gains the subscription `limits` block (additive).
- `supervisor-agent`: the playbook's status report includes the window line.

## Impact

- New: `internal/limits`, `cmd limits`, `limits` in the usage report + config
  keys (`limitsCommand`, `limitsTTLMin`, `limitsWarnPct`).
- Cost/consent: it periodically runs `claude -p /usage` (no tokens — a client
  command — but a process spawn + a network fetch). TTL-gated; disableable with
  `limitsCommand: ""`. The known env wrinkle (home double-VPN needs the proxy
  prefix) is handled by the configurable command string.
- cgo-free; the parse is pure + unit-tested against captured fixtures.
