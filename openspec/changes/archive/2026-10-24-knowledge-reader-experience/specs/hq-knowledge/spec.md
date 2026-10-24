## ADDED Requirements

### Requirement: Knowledge management explains recorded use without internal terminology

Mac and mobile SHALL show recorded use and processing state without opening settings. Ordinary entries SHALL show HQ reference, distinct from standing HQ rules. Unknown legacy audiences SHALL remain unknown. Application records SHALL NOT imply agent loading or product releases. Metadata SHALL be localized, retaining custom keys. IDs and destinations SHALL be available in a compact disclosure. Pending actions SHALL refuse duplicate submission.

#### Scenario: An ordinary entry is read without opening settings
- **WHEN** the reader opens an entry with no audience or application history
- **THEN** HQ reference and its on-demand meaning are visible, without opening an audience dialog

#### Scenario: A plan is not evidence of loading
- **WHEN** a local-agent plan is pending or its application was recorded
- **THEN** the interface distinguishes these states and does not claim running agents loaded the content

#### Scenario: A reader completes a non-removal action
- **WHEN** a plan, instruction write or result record succeeds
- **THEN** the entry remains open for its next step or updated record, while retirement returns to the index

#### Scenario: A phone write is already in progress
- **WHEN** the submit action is triggered again before its first request completes
- **THEN** only one mutation is sent and the interface shows processing until completion or failure

## MODIFIED Requirements

### Requirement: The commander's own detail is recorded only after they confirm, and stays on the machine

Marking an entry sensitive or removing its sensitive mark SHALL require explicit confirmation.

An entry MAY be marked sensitive (`add`/`supersede --sensitive`, or `sensitive <id>`); every
such write and every unmark (`--off`) SHALL require `--confirmed "<the commander's own words>"`,
stored on the entry. A superseded sensitive entry SHALL pass the mark and the words to its
successor. `promote` SHALL refuse a sensitive entry for any audience but `hq`; `machine.md`
and repo blocks SHALL never render one; the API row SHALL carry `sensitive`. `lint` SHALL
report an entry that reads like a credential (bearer token, `password=`/`token=` value, PEM
key, common vendor key prefixes) without the mark as `unmarked-sensitive`, and SHALL NOT
flag a mere mention of the word. The charter SHALL instruct HQ to show the exact title and
body and get the commander's explicit yes before a sensitive write, never from a worker's
capture or a wake, and to keep other people's secrets out entirely.

#### Scenario: Recording an account the commander asked to keep

- **WHEN** HQ runs `add --topic accounts --sensitive --confirmed "可以，记下"` after showing the entry
- **THEN** the entry is live, marked sensitive with those words, and `promote --for machine`
  on it is refused while `--for hq` is allowed

#### Scenario: A credential written without asking

- **WHEN** a live entry carries `token=abcdef123456` and no sensitive mark
- **THEN** lint reports `unmarked-sensitive` on it, and an entry that only says "reset your
  password" is not reported
