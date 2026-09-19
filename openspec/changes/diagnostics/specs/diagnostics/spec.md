# diagnostics (delta)

## ADDED Requirements

### Requirement: Records are status, log, journal or user data, each with one reader

Every file gtmux writes about itself SHALL be one of four kinds: status (what is true now,
one JSON object per component, overwritten), log (what happened, for someone debugging,
JSON Lines), the journal (`events.jsonl`, for HQ) or user data. A surface SHALL learn state
only from status and SHALL NOT parse a log, its own or a third party's, for state.

#### Scenario: A surface needs the tunnel's state

- **WHEN** the menu bar or `gtmux doctor` needs to know whether the tunnel is connected
- **THEN** it reads `status/tunnel.json` and does not read `tunnel.log` or any other log

### Requirement: One log schema across components

A log line SHALL be one JSON object with `ts` (RFC 3339, milliseconds, local offset),
`level` (`debug`, `info`, `warn`, `error`), `component`, `event` (a stable dotted name),
`msg` and `attrs` (a flat map of scalars), written to `logs/<component>.jsonl`. `event`
SHALL be the stable part a test or a bug report refers to. Log lines SHALL NOT carry user
content: no prompt or message text, no pane screen, no upload names.

#### Scenario: A rejected pairing code is recorded

- **WHEN** serve rejects a pairing code
- **THEN** `logs/serve.jsonl` gains a `warn` line with event `enroll.rejected` and the
  reason, and the line does not contain the code

### Requirement: Credentials are redacted where logs are written

The logger SHALL replace, in every line it writes: every secret registered with it (the
serve token, relay token, Direct secret, device tokens as issued); the value of any attr
whose key names a credential; pairing and share URL fragments; and `Authorization` values.

#### Scenario: A call site logs a token by mistake

- **WHEN** a component logs a message or attr that contains the serve token
- **THEN** the written line contains a redaction marker in its place and the token does
  not appear in the file

### Requirement: Logs are bounded by rotation, never truncated in place

Each component's log SHALL rotate by rename when it reaches 2 MB, keeping two older
generations, so a follower does not lose its place.

#### Scenario: A busy component reaches the cap

- **WHEN** `logs/serve.jsonl` reaches 2 MB
- **THEN** it is renamed to `serve.jsonl.1` (the previous `.1` to `.2`, the previous `.2`
  removed) and new lines go to a fresh `serve.jsonl`

### Requirement: Status says when it was written and when it goes stale

A status file SHALL carry `component`, `updated`, `staleAfter` (seconds), `pid`,
`version`, `state`, `since` and `detail`, SHALL be written atomically, and SHALL be
rewritten at least every `staleAfter / 2` while its writer runs. A reader SHALL treat a
status older than `staleAfter` as unknown.

#### Scenario: The writer died

- **WHEN** the tunnel client exits without cleaning up and its status is 5 minutes old with
  `staleAfter` 90
- **THEN** readers report the tunnel's state as unknown, not as the last state written

### Requirement: Tunnel status for both backends

`status/tunnel.json` SHALL report the active backend and whether it is connected. Under
Direct the tunnel client SHALL write it from its connection and from a probe of its own
pairing URL at least every 30 seconds. Under Standard serve SHALL write it from
cloudflared's `cloudflared_tunnel_ha_connections` metric, read from a metrics address
gtmux passes to cloudflared explicitly.

#### Scenario: Direct cannot resolve its server

- **WHEN** the Mac's network cannot resolve the Direct server's name
- **THEN** `status/tunnel.json` reports backend `direct`, state `down`, and the resolver's
  error as the last error

#### Scenario: Standard is registered

- **WHEN** cloudflared reports one or more registered connections
- **THEN** `status/tunnel.json` reports backend `standard`, state `connected`

### Requirement: gtmux's own files are readable by their owner only

Files gtmux owns under `~/.local/share/gtmux/` and `~/.config/gtmux/` SHALL be created
0600 and directories 0700. On the first start of a version that includes this requirement,
gtmux SHALL narrow the existing tree to those modes once, leaving files it does not own
(an agent's settings, the user's instruction files, LaunchAgent plists) at their existing
mode, and SHALL remove the retired `hq-feed/spool.jsonl`.

#### Scenario: An existing world-readable journal

- **WHEN** a version with this requirement first starts and `events.jsonl` is 0644
- **THEN** `events.jsonl` is 0600 afterwards and its contents are unchanged

### Requirement: serve records its own life

serve SHALL log, at `info` or above by default: its start and stop; rejected
authentication, aggregated to at most one line a minute with a count; pairing codes minted
(without the code), redeemed (with the device id) and rejected (with the reason); devices
revoked; failed sends and pushes; and recovered handler panics. It SHALL publish
`status/serve.json` with its boot id, port, version and tunnel backend.

#### Scenario: A scanner hits the tunnel with bad tokens

- **WHEN** 500 requests with unknown tokens arrive within one minute
- **THEN** `logs/serve.jsonl` gains at most one `auth.rejected` line for that minute,
  carrying the count

### Requirement: Nothing leaves the machine unless the user exports it

gtmux SHALL NOT upload logs, status or crash reports anywhere and SHALL NOT include a
third-party telemetry or crash SDK. Logs leave the machine only through an explicit export
(`gtmux doctor --bundle`, or the phone's share action), which SHALL exclude the journal and
user data unless the user opts the journal in.

#### Scenario: A bug report bundle

- **WHEN** the user runs `gtmux doctor --bundle report.tgz`
- **THEN** the archive holds `logs/`, `status/`, the doctor report and version information,
  no `events.jsonl` and no uploads, and the command lists what it included
