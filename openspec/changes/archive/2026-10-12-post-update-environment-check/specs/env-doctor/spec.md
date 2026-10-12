## ADDED Requirements

### Requirement: A read-only structured environment report
The system SHALL expose doctor --json with schema version, CLI version, check timestamp, grouped status/label/value/note rows and status counts. JSON mode SHALL never prompt or apply fixes and SHALL reject repair/bundle options before probing. doctor --no-prompt SHALL also keep human-readable checks read-only on an interactive terminal.

#### Scenario: Blocking JSON report
- **WHEN** a read-only JSON check finds a blocking item
- **THEN** it returns the full parseable report and a nonzero health exit status without applying changes

### Requirement: New versions own post-update checks
After a successful CLI update, the system SHALL execute a bounded, no-prompt doctor check through the newly installed executable. A health finding or failed check SHALL not be presented as installation failure. Menu-bar updates SHALL hand checking to the relaunched app, which SHALL automatically check once per app version, retain a valid report, display checking/results and offer retry.

#### Scenario: Detached menu update
- **WHEN** the old menu-bar process is replaced during installation
- **THEN** the new version performs and presents its environment check instead of depending on the terminated process

### Requirement: A reviewable repair entry
The menu bar SHALL present grouped environment findings and offer doctor --fix in an interactive terminal with one explicit action. It SHALL explain that each repair is confirmed in that terminal, SHALL never add --yes and SHALL let the user recheck after repairs. Recommendations without an automatic fix SHALL remain findings, never be described as automatically repairable.

#### Scenario: Review repair actions
- **WHEN** the user selects Repair environment
- **THEN** the existing doctor fixer explains and confirms each action; launching the terminal does not mark findings repaired
