## ADDED Requirements

### Requirement: Composer region ignores auxiliary status drawers below a prompt box

When an agent TUI renders a boxed status or background-task drawer directly beneath its boxed prompt composer, `SplitInputRegion` SHALL select the upper prompt-bearing box as the draft region whenever the lower box contains no prompt marker and the adjacent upper box begins with a recognized prompt marker.

#### Scenario: Empty composer above a running background-task drawer

- **WHEN** a pane renders an empty `> ` composer box immediately above a boxed background-task row (`● [19:56:02] ... running`)
- **THEN** `DraftOfColored` returns an empty structured draft (`draft == ""`, `structured == true`) and `BoxEmpty` permits delivery

#### Scenario: Unsubmitted user draft above a running background-task drawer

- **WHEN** a pane renders `> draft message` inside the upper composer box above a boxed background-task row
- **THEN** `SplitInputRegion` extracts `"draft message"` as the draft without including the background-task drawer text, and `BoxEmpty` reports `ReachDraft`
