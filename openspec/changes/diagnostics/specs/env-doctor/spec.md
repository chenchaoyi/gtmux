# env-doctor (delta)

## ADDED Requirements

### Requirement: Doctor checks the logs and everything else that grows

`gtmux doctor` SHALL include a `Logs` section with: the log store's size against its cap
and its oldest day against the retention window (flagged when over the cap, or when days
older than retention plus two remain, meaning cleanup is not running); a runaway writer
(flagged when a day passed the in-day cap in the last 7 days, naming the component and
event); recent errors (counts of `error` entries in the last 24 hours by component and
event); file modes (flagged when any gtmux-owned file is wider than 0600 or directory
wider than 0700, with a count and an example path); and every other growing store against
its written bound. `gtmux doctor --fix` SHALL run the log cleanup, narrow modes, and remove
retired files, each recorded as an action.

#### Scenario: Cleanup has stopped

- **WHEN** the log store holds a day file 40 days old and retention is 30 days
- **THEN** the log store row is flagged as not being cleaned, and `gtmux doctor --fix`
  deletes the expired days and records an `act.cleanup` entry

#### Scenario: A world-readable file reappears

- **WHEN** a gtmux-owned file under `~/.local/share/gtmux/` is 0644
- **THEN** the file modes row is flagged with the count and that path, and `--fix` narrows
  it to 0600

### Requirement: Doctor reports the tunnel from its status

`gtmux doctor`'s remote-access section SHALL include a tunnel row read from
`status/tunnel.json`: the backend, whether it is connected, since when, and the last error.
The cloudflared row SHALL apply only when the backend is Standard. The serve row SHALL
claim that a phone can reach this Mac only when the tunnel is connected (Anywhere) or the
door is LAN.

#### Scenario: Direct is down

- **WHEN** the backend is Direct and `status/tunnel.json` reports `down` with an error
- **THEN** the tunnel row is flagged and shows the error, and the serve row does not say
  that the phone can reach this Mac

### Requirement: Doctor can pack a bug report

`gtmux doctor --bundle <path>` SHALL write an archive of `logs/`, `status/`, the doctor
report and version information, SHALL exclude the journal and user data unless
`--with-events` is given, and SHALL print what it included.

#### Scenario: Default bundle

- **WHEN** the user runs `gtmux doctor --bundle out.tgz`
- **THEN** `out.tgz` contains no `events.jsonl` and no uploads, and the command lists the
  files it packed
