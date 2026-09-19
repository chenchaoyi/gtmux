# env-doctor (delta)

## ADDED Requirements

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

#### Scenario: A Direct user is not told about cloudflared

- **WHEN** the backend is Direct
- **THEN** the cloudflared row is absent or marked not used, rather than reported as a
  requirement

### Requirement: Doctor can pack a bug report

`gtmux doctor --bundle <path>` SHALL write an archive of `logs/`, `status/`, the doctor
report and version information, SHALL exclude the journal and user data unless
`--with-events` is given, and SHALL print what it included.

#### Scenario: Default bundle

- **WHEN** the user runs `gtmux doctor --bundle out.tgz`
- **THEN** `out.tgz` contains no `events.jsonl` and no uploads, and the command lists the
  files it packed
