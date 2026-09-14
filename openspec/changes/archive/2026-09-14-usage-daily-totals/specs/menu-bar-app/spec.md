## ADDED Requirements

### Requirement: The reader's Usage tab shows tokens by day

The Usage tab SHALL show, in place of the per-agent lifetime totals, the same tokens
block as the phone: today's and this week's totals, a seven-day bar chart with today in
the stronger ink and direct labels on today and the tallest day, and the week's split per
agent; absent when the CLI carries no `history`.

#### Scenario: The same week on the Mac

- **WHEN** `gtmux usage --json` reports the week above
- **THEN** the Usage tab's tokens block reads the same two figures and draws the same
  seven bars
