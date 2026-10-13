## ADDED Requirements

### Requirement: Upgrade notes are readable once and revisitable
The menu-bar app SHALL show a compact, resizable What's new window after an
upgrade when the user first opens the menu, following the phone's newest-expanded/older-folded version grouping.
Hidden background launches SHALL NOT activate the reading window; a response after
the menu closes SHALL wait for the next opening.
It SHALL show only notes up to the installed app version and provide permanent
gear-menu and Software update preferences entries. Prose SHALL wrap to the actual
window width and scroll; Return, Escape and window close SHALL dismiss the reader.
It SHALL acknowledge successfully read notes on dismissal, not on fetch start.
First installation SHALL be quiet; downgrade SHALL not lower acknowledgement.
Missing acknowledgement on an existing install SHALL migrate the prior launch
version before the environment check replaces that marker.

#### Scenario: Skip several releases
- **WHEN** the installed app is newer than the acknowledged version
- **THEN** the automatic reader includes each unseen version in the latest-30-release
  source range, newest first, without including releases newer than the installed app
- **AND** a source-range gap is disclosed with a release-archive entry

#### Scenario: Fresh install or acknowledged version
- **WHEN** there is no prior launch/acknowledgement, or the current version is at or below it
- **THEN** no automatic reader is shown; the manual entry remains available

#### Scenario: Read fails or is superseded
- **WHEN** the CLI times out, fails or returns invalid version/schema data
- **THEN** acknowledgement is unchanged and manual reading provides Retry
- **AND** a superseded automatic reply cannot replace a manual history request

#### Scenario: Reopen notes offline
- **WHEN** a valid bilingual receipt for the installed version is cached
- **THEN** the manual reader opens without a network request and changes language
  using the selected block, falling back to the other language when absent

### Requirement: Structured notes reuse the core's authored blocks
`gtmux whatsnew --json` SHALL return schema 1 with `since`, `until`, `limit` (30),
`published_through`, `oldest_available` and newest-first `entries` carrying `version`,
`en` and `zh`. JSON SHALL exclude drafts, prereleases and generated commit subjects.
`--until` SHALL supply an inclusive upper bound; the existing text command SHALL
retain its open upper bound when that flag is absent. Failed reads SHALL return
nonzero rather than an empty successful receipt.

#### Scenario: Installed-version read
- **WHEN** the desktop requests `whatsnew --all --until <installed> --json`
- **THEN** both authored language blocks are returned only for stable releases at or
  below that version, with empty arrays for missing languages

#### Scenario: Existing CLI behavior
- **WHEN** text notes are requested without `--until`
- **THEN** newer published versions remain included as before
