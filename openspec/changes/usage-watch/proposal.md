# usage-watch — token usage, layered thresholds, and ahead-of-time warnings

## Why

The supervisor (HQ) watches WHAT agents are doing but not what they are
SPENDING. The user runs 10+ concurrent agents on a subscription with real
limits; today the first sign of trouble is an agent hitting a wall (context
compact, rate limit) mid-task. Ask (2026-07-12): HQ 整体关注 token usage、
各类 agent 的各层阈值、按使用速度评估将超阈值时提前警示.

Deterministic source confirmed: every Claude assistant message logs
`usage{input_tokens, output_tokens, cache_creation_input_tokens,
cache_read_input_tokens}` + a timestamp in the session jsonl — enough to compute
per-session totals, CONTEXT pressure, spend RATE, and projections with zero
LLM calls, in keeping with the digest layer's cost model.

## What Changes

- **New capability `usage-watch`** (P1, Claude-first like every transcript
  feature):
  - `internal/usage`: per-session extraction from the transcript tail —
    cumulative in/out tokens, last-message context footprint (input +
    cache_read + cache_creation ≈ live context), context% against the model
    window, and a sliding-window RATE (output-tokens/min over the last 10
    minutes, timestamp-based).
  - **Layered thresholds** in `~/.config/gtmux/usage.json`, defaulting
    sensibly and overridable PER AGENT TYPE:
    `{"claude": {"ctxWarn": 0.8, "ratePerMinWarn": 8000, "sessionTokWarn": 2e6}, …}`
    Layers: per-session context% · per-session total burn · per-agent-type
    aggregate burn rate (all sessions of that agent summed).
  - **Projection**: `willExceed = current + rate × horizon` (default 30 min)
    against each threshold → a WARN state BEFORE the wall, not at it.
  - Surfacing: `gtmux usage [--json]` (per-session + per-agent-type rollup);
    digest rows gain `tok`, `ctx` (0–1), `rate`, and `usage_warn` (the first
    breached/projected layer, e.g. "ctx@86%" / "rate→session cap in ~12m");
    `GET /api/usage` additive.
  - **Warnings**: the radar marks a usage-warned row (amber modifier — like
    errored/bg, a modifier not a status); the existing hook nudge channel
    gains a `[gtmux] usage·warn <loc> — <detail>` line to a live HQ (same
    dedup pattern); the HQ playbook gains a usage policy section (watch
    `gtmux usage --json`, advise compact/split/pause).
- **P2 (spec'd deferred)**: subscription-window awareness (Max 5h/weekly
  windows) if a reliable local source exists; $ cost estimation; Codex usage
  parsing; menu-bar/mobile usage badges.

## Capabilities

### New Capabilities
- `usage-watch`: extraction, thresholds, projection, CLI/API, warn channel.

### Modified Capabilities
- `agent-digest`: rows gain the additive usage fields.
- `supervisor-agent`: the playbook's usage policy + the usage nudge line.

## Impact

- New: `internal/usage`, `cmd usage`, `/api/usage`, `usage.json` config.
- Touched: digest assembly (fields), hook nudge (usage line), hq playbook
  (seeded section — NOTE: existing seeded homes are never overwritten; the
  release notes tell users to append the usage section or re-seed).
- Zero LLM tokens; cgo-free; transcript-tail incremental (same loader).
