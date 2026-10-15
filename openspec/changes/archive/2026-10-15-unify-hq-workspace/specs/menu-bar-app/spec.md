## MODIFIED Requirements

### Requirement: The supervisor's two memories are readable, and the knowledge base is judgeable, at the Mac

The HQ card SHALL open situation board and knowledge as named destinations in the
same retained HQ workspace window, rather than separate readers or content inside the
popover. Knowledge SHALL show what it owes the commander at the top. Both SHALL read
exclusively through the CLI (`gtmux hq --board --json`,
`gtmux knowledge list --json`, `gtmux capture --list --json`), so the app stays a pure
consumer and never resolves the HQ home itself; that path is relocatable and symlinked on
real machines, so it SHALL be asked for (`gtmux hq --home`) rather than rebuilt.

Reader shortcut icons SHALL identify their destination: an open book for Knowledge,
a document for the situation board. The Knowledge metaphor SHALL match the phone/iPad
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
- **AND** prose before the table and explicitly headed content remain directly visible
- **AND** empty entries are omitted; a decision body lifted to Needs your input appears
  only once, with any child entries remaining accessible and counts excluding hidden entries
- **AND** sections, entries and pane rows use consistent 13-point SF chevrons with at
  least 32-point hit areas and accessible expanded states; long prose has labelled
  Read full text and Collapse text actions
- **AND** pane links and table disclosure remain separate actions; reading the board
  does not rewrite HQ records or turn its snapshot into live radar state

#### Scenario: Opening a pane from the Mac situation board

- **WHEN** board prose or a table cell contains a standalone tmux pane ID such as `%29`
- **THEN** it appears as an underlined native link, including inline-code and bold references
- **AND WHEN** the user clicks it
- **THEN** the app runs `gtmux focus %29` off the UI thread, using the core's existing
  terminal selection and detached-session attachment behavior
- **AND** table disclosure remains a separate action; opening a pane does not expand its row
- **AND WHEN** focus fails, including a closed pane or an unavailable terminal
- **THEN** the app shows a localized error with the CLI reason and allows retry
- **AND** names, paths, URL escapes, fenced code and knowledge documents do not acquire
  pane actions; links accept only one canonical pane target, never arbitrary CLI arguments

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

### Requirement: The reader window shows usage

The HQ workspace SHALL offer a Usage sidebar destination reading `gtmux usage --json`: the tightest
non-session window and its reset as a lead, quotas grouped by agent as bars coloured by
each window's tier (blue, amber when the core warns, red at 100%, with no separate warning
line),
per-agent output with the note that it is not a billing period, and sessions sorted by
trouble (alerted, then burn rate, then context share) with parked sessions folded into a
count. It SHALL poll only while that destination is visible in the workspace.

#### Scenario: Where am I standing

- **WHEN** the plan windows read claude session 30%, claude week (all models) 29%, claude
  week (fable) 49%, codex week 0%
- **THEN** the card's usage row reads "claude Fable 49% · codex wk 0%" and the Usage destination
  leads with the fable week at 49%

### Requirement: The reader window shows the machine

The HQ workspace SHALL offer a Machine sidebar destination reading `gtmux resource --json`:
the four readings (memory, disk, load, power), the core's own warning sentence, the
per-agent RSS/CPU table heaviest first with each pane's session name, and the orphan
processes the core calls reclaimable with the core's own hint. The destination SHALL be read-only:
it SHALL offer no kill or reclaim action. It SHALL poll only while that destination is visible in the workspace.

#### Scenario: Reading a red tier

- **WHEN** the machine is at the red tier because memory is critical
- **THEN** the Machine destination shows the memory reading marked critical, the core's warning
  sentence, and the heaviest agent process in the attention colour, with no button that
  ends a process

### Requirement: The Mac export asks for its lock first, then the place, then confirms

Export… in the workspace’s Knowledge archive area SHALL open a sheet that asks for the passphrase (two fields, a
show toggle, and one hint line naming the one thing to fix) before the save panel, SHALL
offer to keep the passphrase in this Mac's keychain so the next export does not ask, SHALL
show a remembered passphrase as a single line with a way to change it, SHALL hand the
passphrase to the CLI over stdin and never on the command line, and SHALL end on a
confirmation page carrying the written path, its size, and how the file opens — or, on
failure, the CLI's own words. The archive label’s help SHALL show when the last export was made and whether it
was encrypted, together with archive size, local snapshots and protection status.

#### Scenario: First export on a Mac

- **WHEN** the commander clicks Export… with no passphrase in the keychain, types one of
  twelve characters twice and leaves "remember" on
- **THEN** the hint reads "good", the save panel offers `gtmux-hq-<date>.tar.gz.age`, the
  file is written locked, the passphrase is stored in the keychain, and the sheet ends on
  the path, the size and the sentence saying how it opens

#### Scenario: Second export

- **WHEN** the commander clicks Export… again
- **THEN** the sheet shows one line saying the keychain's passphrase will be used, with
  "Change…", and the export is one click away
