## ADDED Requirements

### Requirement: The HQ card expands into the chief-of-staff report

The HQ card SHALL carry a disclosure at the right end of its head that opens, inside the
card's bordered panel, the same report table the phone's HQ page expands into (MOBILE
§17): a key column and a value column, one row per question — `machine` (readings; a door
to the reader's machine tab) · `knowledge` (entry count, what it owes the commander and
the oldest debt; a door) · `board` (how fresh; a door) · `HQ did` (the last day's tally of
the supervision's own acts, from `gtmux events --since 24h --acts`, in the phone's fixed
order). The head's click SHALL still focus the supervisor's pane. Keys, values and verbs
SHALL follow the app's language.

Rows SHALL follow the phone's rules: a row with nothing to say is absent, never a zero;
the machine row leads, in the attention colour and allowed a second line, ONLY at the red
tier, keeps its ordinary place in amber at the amber tier, and is plain otherwise; the
knowledge row alone may turn amber, and only when the oldest promotion has waited past the
two-week line `gtmux doctor` uses.

The expansion SHALL open itself once on ENTERING an attention state (the supervisor
waiting, a worker waiting, or a red resource tier) and never close itself; a manual toggle
SHALL be remembered across popover openings. Its data SHALL be read through the CLI only
while the expansion is showing in an open popover, and the events read SHALL run from the
app's own working directory, never the HQ home, so it cannot advance HQ's consumption
watermark.

#### Scenario: A red machine explains itself on the card

- **WHEN** `gtmux resource --json` reports `machine.tier` = `red` while the card was
  collapsed
- **THEN** the card opens itself, the machine row is first and red, and it reads the
  memory, disk, load and reclaimable-orphan figures, with a door to the machine tab

#### Scenario: The knowledge debt is on the card

- **WHEN** 7 promotions await the commander and the oldest has waited 16 days
- **THEN** the knowledge row reads the entry count, "7 waiting on you" and "oldest 16d"
  (「7 条待你带走 · 最久 16 天」 in Chinese) in amber, and opens the knowledge tab

#### Scenario: All normal stays one line

- **WHEN** nothing is waiting, the machine is healthy and the reader has not opened the
  report
- **THEN** the card shows the medallion and the headline only, with the disclosure closed

### Requirement: The reader window shows the machine

The HQ reader window SHALL offer a third tab, Machine, reading `gtmux resource --json`:
the four readings (memory, disk, load, power), the core's own warning sentence, the
per-agent RSS/CPU table heaviest first with each pane's session name, and the orphan
processes the core calls reclaimable with the core's own hint. The tab SHALL be read-only:
it SHALL offer no kill or reclaim action. It SHALL poll only while it is the showing tab.

#### Scenario: Reading a red tier

- **WHEN** the machine is at the red tier because memory is critical
- **THEN** the machine tab shows the memory reading marked critical, the core's warning
  sentence, and the heaviest agent process in the attention colour, with no button that
  ends a process
