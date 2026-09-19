# diagnostics (delta)

Phases 1 and 2 were synced into `openspec/specs/diagnostics/spec.md` as they landed;
phase 3 is below.

## ADDED Requirements

### Requirement: Entries leave the machine only through an explicit export

Entries SHALL leave the machine only through an explicit export: `gtmux doctor --bundle`,
or the phone's share action on its own diagnostics buffer. An export SHALL exclude the
journal and user data unless the user opts the journal in.

#### Scenario: A bug report bundle

- **WHEN** the user runs `gtmux doctor --bundle report.tgz`
- **THEN** the archive holds `logs/`, `status/`, the doctor report and version information,
  no `events.jsonl` and no uploads, and the command lists what it included

### Requirement: The phone keeps a bounded diagnostics buffer

The phone and iPad app SHALL keep the last 500 entries, at most 200 KB, in the store's
schema with component `phone`: failed requests to the Mac by route (the first failure,
repeats within a minute as a count, and the first success after), each pairing attempt
with its outcome and reason, push registration, and the live stream dropping and coming
back. Credentials SHALL be replaced where an entry is written: every paired Mac's token,
a pairing code being redeemed, pairing and share fragments, `Authorization` and bearer
values, and credential-named attributes. The buffer SHALL persist across launches, stay
on the device, and leave it only from Settings → Diagnostics, whose Copy and Share hand
over a header line and the entries as JSON lines.

#### Scenario: A dead Mac does not push out the pairing that explains it

- **WHEN** the Mac stops answering and the app polls it every few seconds for ten minutes
- **THEN** the buffer gains about one failure entry a minute per route, and the pairing
  attempt recorded before it is still there

