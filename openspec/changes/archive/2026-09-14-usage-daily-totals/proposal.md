# Change: usage-daily-totals

> STATUS: implemented 2026-09-14 (same PR). On the commander's direction, pointing at the phone's usage sheet: 「这一块的信息能不能换为每天每周的所有 agent 的 token 使用量之和？展示上可以设计一下」.

## Why

`gtmux usage` counted output per SESSION since that session began. The phone's "output so
far" block therefore had to carry a sentence explaining that 69.0M was not a billing
period, and the Mac's new Usage tab inherited the same block. The number people ask is
simpler: how much did all my agents burn today, and this week.

## What Changes

1. **A daily token ledger in the core** (`internal/usage/daily.go`): every transcript on
   the machine that changed in the last eight days is read from a per-file byte
   watermark; each usage message is attributed by ITS OWN timestamp to the local day it
   happened; cumulative logs (Codex) contribute deltas; a file lock serialises writers.
   `gtmux usage --json` / `GET /api/usage` gain `history`: the last seven days oldest
   first, `today_out` / `week_out` (and the input pair), and the week split by agent with
   the registry's display names. The CLI prints one `Σ today … · this week …` line.
2. **The "output so far" block becomes a tokens block on the phone, the iPad and the
   Mac**: two hero numbers (today, this week), a seven-day bar chart — a single neutral
   series, thin bars, today's bar in the stronger ink, direct labels on today and the
   tallest day, weekday initials underneath — and the week's split per agent. The
   per-session list below is unchanged.

## Surfaces

- 终端 (terminal, incl. remote attach): `gtmux usage` prints the today/week line;
  `--json` carries `history`.
- 菜单栏 (menubar): the reader's Usage tab shows the tokens block in place of the
  per-agent lifetime totals.
- 手机 (phone): UsageSheet's "output so far" becomes the tokens block.
- iPad: the same sheet.
- Web: not applicable — the shared page has no usage view.

## What does NOT change

- Per-session rows, the plan windows, the machine section, thresholds and warnings.
- No model is called; nothing leaves the machine. The ledger is a few KB under the state
  directory.
