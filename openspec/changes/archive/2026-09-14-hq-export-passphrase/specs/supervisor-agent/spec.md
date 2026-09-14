## ADDED Requirements

### Requirement: The memory export is locked with a passphrase

`gtmux hq --export` SHALL write the memory as a passphrase-locked age file (`.tar.gz.age`,
adding the suffix when the given name lacks it) unless `--plain` is passed. The passphrase
SHALL be taken from the first line of stdin with `--passphrase-stdin`, else from
`GTMUX_HQ_PASSPHRASE`, else from an unechoed terminal prompt typed twice; it SHALL be at
least eight characters, and a mismatch or a short one SHALL ask again rather than fail. It
SHALL never be accepted on the command line. `gtmux hq --import` SHALL recognise a locked
export, take the passphrase the same ways, and refuse a wrong one before moving or writing
anything. `gtmux hq --memory` SHALL report when the last export was made and whether it was
locked, and SHALL say nothing when there has never been one.

#### Scenario: A locked export comes back only with its passphrase

- **WHEN** the memory is exported with a passphrase and the HQ home is then deleted
- **THEN** importing the file with that passphrase restores the board, the knowledge base
  and `LOCAL.md`, and importing it with another passphrase is refused with the existing
  memory untouched

#### Scenario: A short passphrase is refused before anything is written

- **WHEN** an export is given a seven-character passphrase
- **THEN** no file is written and the error names the floor
