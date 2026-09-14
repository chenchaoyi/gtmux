## ADDED Requirements

### Requirement: The Mac export asks for its lock first, then the place, then confirms

The reader window's Export… SHALL open a sheet that asks for the passphrase (two fields, a
show toggle, and one hint line naming the one thing to fix) before the save panel, SHALL
offer to keep the passphrase in this Mac's keychain so the next export does not ask, SHALL
show a remembered passphrase as a single line with a way to change it, SHALL hand the
passphrase to the CLI over stdin and never on the command line, and SHALL end on a
confirmation page carrying the written path, its size, and how the file opens — or, on
failure, the CLI's own words. The memory line SHALL show when the last export was made and
whether it was locked.

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
