## ADDED Requirements

### Requirement: A decision on the board can be given to HQ from the item itself

The situation board sheet SHALL render the commander's section (「还等你定的」 / "Still
waiting on you") as its numbered items, each a row under its group heading with a "Tell
HQ" affordance. Tapping an item SHALL offer "Do as you suggest" when the item carries a
recommendation, and "Let me say…" always. "Do as you suggest" SHALL send to HQ's pane a
reply that names the item by HQ's number and first line and accepts the recommendation;
"Let me say…" SHALL close the sheet and place that quote in the composer for the commander
to finish. Without a handler the rows SHALL be read-only. A section with no numbered items
SHALL render as before.

#### Scenario: Taking HQ's recommendation

- **WHEN** the commander taps item 1 (「折中还是纯指路 —— 我建议折中…」) and chooses "Do as you suggest"
- **THEN** HQ receives 「态势板「还等你定的」第 1 条（折中还是纯指路 —— 我建议折中。…）：按你的建议办。」

#### Scenario: Saying it himself

- **WHEN** the commander taps item 5 (no recommendation) — only "Let me say…" is offered — and chooses it
- **THEN** the sheet closes and the composer holds 「态势板「还等你定的」第 5 条（…）：」 with the cursor after it
