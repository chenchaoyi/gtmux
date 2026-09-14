## ADDED Requirements

### Requirement: The usage sheet shows tokens by day

The usage sheet SHALL replace the per-session lifetime "output so far" block with a
tokens block: today's and this week's totals across every agent as two figures, a
seven-day bar chart (one neutral series, today in the stronger ink, direct labels on
today and the tallest day, weekday initials beneath), and the week's split per agent.
The block SHALL be absent when the serve carries no `history`.

#### Scenario: A week of work

- **WHEN** `history` reports seven days with today at 12.4M and the week at 69.0M
- **THEN** the sheet shows "12.4M today · 69.0M this week", seven bars with today's
  labelled, and one row per agent with its week total
