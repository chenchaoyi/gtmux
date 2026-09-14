## ADDED Requirements

### Requirement: Tokens are totalled by local day across every agent

The system SHALL keep a daily token ledger: each usage message in every agent transcript
on the machine SHALL be attributed by its own timestamp to the local day it happened (a
cumulative log contributing the delta between consecutive totals), read incrementally
from a per-file byte watermark under a file lock, retaining sixty-two days. `gtmux usage
--json` and `GET /api/usage` SHALL carry `history`: the last seven local days oldest
first with per-agent counts, `today_out`/`today_in`, `week_out`/`week_in`, and the
week's split per agent with the registry's display name. `gtmux usage` SHALL print one
today/this-week line.

#### Scenario: Two days of two agents

- **WHEN** a Claude log carries 1,000 output tokens dated yesterday and 2,000 dated today,
  and a Codex log's cumulative totals go 500 → 800 today
- **THEN** `history.today_out` is 2,300, `history.week_out` is 3,300, and the week's split
  reads claude 3,000 · codex 300

#### Scenario: Reading twice counts once

- **WHEN** the ledger is updated, nothing is appended, and it is updated again
- **THEN** the totals are unchanged
