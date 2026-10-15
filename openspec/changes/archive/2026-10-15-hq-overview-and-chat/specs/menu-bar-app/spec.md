## MODIFIED Requirements

### Requirement: The supervisor's two memories are readable, and the knowledge base is judgeable, at the Mac

The HQ card SHALL open Overview and knowledge in the same retained HQ workspace
window, rather than separate readers or content inside the popover. Overview SHALL
use explicit matters for current work and retain the complete authored board as a
secondary, dated Handoff snapshot. The report's handoff row SHALL open that snapshot. Knowledge SHALL show what it owes the commander at the top. Both SHALL read
exclusively through the CLI (`gtmux hq --board --json`,
`gtmux knowledge list --json`, `gtmux capture --list --json`), so the app stays a pure
consumer and never resolves the HQ home itself; that path is relocatable and symlinked on
real machines, so it SHALL be asked for (`gtmux hq --home`) rather than rebuilt.

Reader shortcut icons SHALL identify their destination: an open book for Knowledge,
a grid for Overview, and a document for the Handoff snapshot. The Knowledge metaphor SHALL match the phone/iPad
HQ shortcut; tooltips and accessibility labels SHALL continue to name the destination.
Native symbol styling need not be pixel-identical across platforms.

#### Scenario: The Knowledge shortcut is recognisable across surfaces

- **WHEN** an owner opens the Mac HQ banner or the phone/iPad HQ shortcut menu
- **THEN** Knowledge is represented by a book, and its accessible name identifies the knowledge base

The knowledge reader SHALL additionally offer the JUDGMENT actions — `promote`,
manual `land`, audience-directed carry, `withdraw`, `retire`, `dismiss`, and the
`everyone` audience's feedback link — and SHALL NOT offer the AUTHORING verbs `add`
and `supersede`. The boundary this draws is between judging what is already written and
writing new prose. Resource destinations SHALL NOT dispatch or send instructions;
scoped HQ instructions belong to the workspace conversation/matter composer.

Each CLI-backed act SHALL:

- run the corresponding CLI verb from the HQ home, so the ledger — not a second
  implementation in the app — decides what it means and whether it is allowed;
  carry runs `knowledge land <id>` without `--ref`;
- require a REASON for `promote`, manual `land`, `withdraw`, `retire` and `dismiss`
  (`--why`, or `--ref` for manual `land`), and take exactly ONE confirmation naming
  the act and its subject before running; carry needs that confirmation but no reason;
- report a failure as the CLI's own stderr, verbatim and unedited;
- refresh the window's contents on success, so the reader sees the state they created.

The feedback action SHALL open the prefilled issue in a browser, without running a
CLI mutation or publishing the issue.

Which acts an entry offers SHALL follow its audience and promotion lifecycle:
a pending `hq` / `machine` / `repo` promotion offers carry, `withdraw` and `retire`;
a pending `everyone` promotion offers feedback when its issue URL is present, manual
`land`, `withdraw` and `retire`; an old promotion without an audience offers manual
`land`, `withdraw` and `retire`. Any other live entry offers `promote` and `retire`.
Candidates SHALL be grouped by dedup key, because `dismiss --capture <key>` consumes
every pending line sharing it.

The candidate section SHALL identify these as unfiled leads for HQ to verify and
distil, not as a user approval queue. It SHALL explain that dismissal is an
optional correction for an incorrect or redundant lead. Long leads SHALL be
represented by at most two preview lines in the left index. Selecting a lead SHALL
show its complete, selectable original text and dismissal action in the right
detail pane, with a visible selection mark in the index. The detail SHALL scroll
independently; index rows SHALL NOT expand or offer dismissal inline. Refreshes
SHALL preserve selection by capture key and update changed lead text or metadata;
a lead removed by HQ or dismissed elsewhere SHALL show an unavailable state.

Form and wording SHALL match the phone's knowledge sheet (MOBILE, `hq-knowledge-on-phone`)
rather than inventing a third dialect: `land` and `retire` reuse its copy, and the acts sit
in the entry's detail view rather than on the index rows a reader is scanning.

#### Scenario: A long pending lead

- **WHEN** HQ has not yet distilled a long capture candidate
- **THEN** the left index shows a two-line preview without overlap or inline actions
- **AND WHEN** the user selects the lead
- **THEN** the right pane shows the complete original text, explains HQ's review,
  and offers dismissal through the existing reason-and-confirmation sheet

#### Scenario: A selected lead changes during review

- **WHEN** a refresh changes a lead's text or grouping without changing its key
- **THEN** its selected detail updates without changing selection
- **AND WHEN** that key leaves the pending queue
- **THEN** the detail says the lead is no longer awaiting review

#### Scenario: Reading the board

- **WHEN** the commander opens the board reader and the supervisor has written one
- **THEN** the document is shown in a resizable window, selectable, laid out lazily so a
  50 KB board opens without a stall
- **AND WHEN** no board has ever been written
- **THEN** the window says so as an ordinary state, not as a failure

#### Scenario: Understanding the board outline

- **WHEN** the Mac reader displays the canonical English or Chinese seeded sections
- **THEN** it labels them Sessions and Handoff log in the app language, without step
  numbers or a sorting instruction; custom headings and document order are preserved
- **AND** unlabelled supplementary blocks after the sessions table appear under HQ notes,
  collapsed by default, with every source block retained in order and full text on expansion
- **AND** each nonempty body and child remains available under its original section
- **AND** sections start collapsed, empty entries are omitted, and old decisions are not
  lifted into a red current-attention block; counts exclude empty entries
- **AND** sections, entries and pane rows use consistent 13-point SF chevrons with at
  least 32-point hit areas and accessible expanded states; long prose has labelled
  Read full text and Collapse text actions
- **AND** reading does not rewrite HQ records or turn its snapshot into live radar state

#### Scenario: Opening a pane from the Mac situation board

- **WHEN** a Handoff snapshot contains a pane number such as `%29`
- **THEN** it remains a plain source reference rather than an actionable current pane link
- **AND** table disclosure still expands its saved fields
- **AND** current controls remain available from a matter's identity-checked linked conversation

#### Scenario: The board's lists render as lists on the Mac as on the phone

- **WHEN** the board carries a numbered list whose items wrap onto indented lines
- **THEN** the Mac shows numbered items, each owning its wrapped lines, exactly as the
  phone does — never the items run together as one paragraph

#### Scenario: Closing out a promotion at the Mac

- **WHEN** the commander opens a promoted entry and confirms "mark it landed" with a ref
- **THEN** `gtmux knowledge land <id> --ref <ref>` runs from the HQ home, the ledger
  records it with the same `gtmux:audit:knowledge` trail any other door would leave, and
  the window re-reads so the entry has left the "waiting on you" section

#### Scenario: A refused act says what the CLI said

- **WHEN** an act is refused (an entry with no pending promotion, an unknown candidate
  key, a machine with no HQ home)
- **THEN** the CLI's own message is shown unedited, and nothing in the ledger changed

#### Scenario: An act that needs a reason cannot run without it

- **WHEN** the confirm sheet for an act that requires a reason is open with an empty
  or blank reason
- **THEN** the confirming action is unavailable and no process is spawned

#### Scenario: Authoring is not offered here

- **WHEN** the commander looks for a way to add or supersede an entry
- **THEN** the window offers neither, and says the writing verbs stay in the CLI

### Requirement: The HQ card expands into the chief-of-staff report

The HQ card SHALL carry a disclosure at the right end of its head that opens, inside the
card's bordered panel, the same report table the phone's HQ page expands into (MOBILE
§17): a key column and a value column, one row per question — `machine` (readings; a door
to the reader's machine tab) · `knowledge` (entry count, what it owes the commander and
the oldest debt; a door) · `handoff` (saved snapshot age; a door) · `usage` (today's and this week's
tokens when the CLI carries `history`, then one window per plan, the tightest, from
`gtmux usage --json`; a door to the reader's Usage tab; absent when neither is readable) · `HQ did` (the last day's tally of
the supervision's own acts, from `gtmux events --since 24h --acts`, in the phone's fixed
order). The head's click SHALL open the retained HQ workspace; Terminal remains an explicit separate action. Keys, values and verbs
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
