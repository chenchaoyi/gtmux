## MODIFIED Requirements

### Requirement: HQ workspace provides readable typography and classified activity
The full HQ window SHALL use a scoped reading type scale rather than compact menu-popover sizes: body and record titles at 15pt, sidebar labels at 14pt, metadata/code at least 13pt, and larger section/page headings. Sidebar category names SHALL not be smaller or fainter than child navigation labels. Markdown, native input, associated conversation cards and archive forms SHALL follow the same hierarchy without changing unrelated windows. Parallel Export/Import controls SHALL use the same 14pt regular-size style. Knowledge disclosures SHALL use standard symbols with accessible expansion state, and wide prose SHALL remain within a readable column.

Existing action kinds SHALL be classified as instruction delivery, updates, matter/conversation organization, supervision, knowledge or maintenance, with an Other fallback. Workspace actions and journal events SHALL share a local-calendar, 24-hour timeline ordered by recorded time, with stable ties that do not assert cross-source causality. Only adjacent workspace actions with the same explicit matter ID within five minutes SHALL be grouped; grouping SHALL preserve every source record. Rows SHALL use semantic symbols, expose the category in accessible text, and support filtering the loaded records by category without changing chronology, joining groups across hidden unrelated records or mutating stored data. Full source text, exact time and IDs SHALL remain inspectable in a bounded sheet. Missing titles/times SHALL NOT be invented. Counts SHALL describe matching loaded source records; a clickable explanation SHALL state source limits and distinguish recorded actions from task completion. Unknown kinds SHALL remain available in All and Other. Failure outcomes SHALL remain distinguishable from successful historical records.

#### Scenario: A reader filters activity
- **WHEN** the reader selects an activity category
- **THEN** matching workspace and journal records remain in unified recorded-time order and the loaded record count is shown
- **AND** returning to All restores every record, including unknown kinds

#### Scenario: Reading a full HQ window
- **WHEN** the reader opens the sidebar, conversation, resources or composer
- **THEN** the scoped reading sizes are used and the compact menu typography remains unchanged

#### Scenario: Related actions are separated by another matter
- **WHEN** two actions on one matter have an unrelated action between them
- **THEN** filtering out the unrelated action does not combine the two groups
- **AND** every remaining source record can still be inspected

#### Scenario: Archive controls and explanations
- **WHEN** the reader opens Knowledge
- **THEN** Export and Import use matching text size and control size
- **AND** clicking HQ archives opens readable archive details without requiring hover

### Requirement: The Mac export asks for its lock first, then the place, then confirms

Export… in the workspace’s Knowledge archive area SHALL open a sheet that asks for the passphrase (two fields, a
show toggle, and one hint line naming the one thing to fix) before the save panel, SHALL
offer to keep the passphrase in this Mac's keychain so the next export does not ask, SHALL
show a remembered passphrase as a single line with a way to change it, SHALL hand the
passphrase to the CLI over stdin and never on the command line, and SHALL end on a
confirmation page carrying the written path, its size, and how the file opens — or, on
failure, the CLI's own words. A clickable archive explanation SHALL show when the last export was made and whether it
was encrypted, together with archive size, local snapshots and the local-storage boundary.

#### Scenario: First export on a Mac

- **WHEN** the commander clicks Export… with no passphrase in the keychain, types one of
  twelve characters twice and leaves "remember" on
- **THEN** the hint indicates the recommended password length, the save panel offers `gtmux-hq-<date>.tar.gz.age`, the
  file is written locked, the passphrase is stored in the keychain, and the sheet ends on
  the path, the size and the sentence saying how it opens

#### Scenario: Second export

- **WHEN** the commander clicks Export… again
- **THEN** the sheet shows one line saying the keychain's passphrase will be used, with
  "Change…", and the export is one click away


### Requirement: The HQ window shows an entry's three axes and carries it

The knowledge tab SHALL show kind, provenance (with count) and audience on an entry as
the four audience words (中控 · 本机 · 仓库 · 全体 / hq · machine · repo · everyone), and
SHALL offer "Apply knowledge" for `hq` / `machine` / `repo` (preview, append, land) and
"Feedback to gtmux" for `everyone`, plus "Cancel sharing" for `withdraw`. Choosing an audience SHALL describe preparation of a sharing brief, not immediate application; recording a destination SHALL retain the existing `land --ref` operation. The pool view SHALL group by
neighbourhood.

#### Scenario: Carrying a machine-wide lesson

- **WHEN** the user confirms "Apply knowledge" on a `machine` entry
- **THEN** the canonical file and every agent block are refreshed, the entry lands with
  the canonical path as ref, and doctor's sync row is green

