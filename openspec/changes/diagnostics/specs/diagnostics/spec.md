# diagnostics (delta)

## ADDED Requirements

### Requirement: One local log store for diagnostics and actions

Every gtmux process SHALL write its log entries to one store under
`~/.local/share/gtmux/logs/`, one file per local day. An entry SHALL be one JSON object
with `ts` (RFC 3339, milliseconds, local offset), `level` (`debug`, `info`, `warn`,
`error`), `component`, `kind` (`diag` or `act`), `event` (a stable dotted name; action
events start with `act.`), `msg` and `attrs` (a flat map of scalars). Entries SHALL be in
English. Each entry SHALL be appended in a single write of at most 4 KB, and a failure to
log SHALL NOT fail the operation that was logging.

#### Scenario: Two processes log at once

- **WHEN** serve and a hook append entries to the same day file at the same moment
- **THEN** both entries are present as whole lines and neither is interleaved with the
  other

### Requirement: Actions record who, what and how it ended

An action entry SHALL carry `actor` (`user`, `agent:%N`, `hq`, `menubar`, `phone:<device>`,
`browser:<device>`, `guest:<link>`, `system`, `update`), `target` and `outcome` (`ok`,
`refused` with a reason, or `failed` with an error). Every act that changes something on
the Mac, whoever started it, SHALL be recorded as an action, including the acts the
journal audits for HQ, which SHALL be written to both through one call.

#### Scenario: A phone types into a pane

- **WHEN** a paired phone sends a message into pane `%7` through serve
- **THEN** the store gains an `act.send` entry with actor `phone:<its device id>`, target
  `%7`, outcome `ok`, and the message's length and a short hash, but not its text

#### Scenario: A command marked as writing has no action

- **WHEN** a command in the command table is marked as writing and the action catalog has
  no event for it
- **THEN** the test suite fails

### Requirement: Entries carry no user content and no credentials

An entry SHALL NOT carry prompt or message text, pane screens, upload file names or
knowledge entry bodies. The writer SHALL replace, in every entry: every secret registered
with it (the serve token, relay token, Direct secret, device tokens as issued); the value
of any attr whose key names a credential; pairing and share URL fragments; and
`Authorization` values.

#### Scenario: A call site logs a token by mistake

- **WHEN** a component logs a message or attr that contains the serve token
- **THEN** the written entry holds a redaction marker in its place and the token does not
  appear in the store

### Requirement: The log store bounds its own growth

The store SHALL delete entries older than 30 days and, while it is over 100 MB, SHALL
delete the oldest days first, never today's file; both limits SHALL be configurable. The
cleanup SHALL run when the first entry of a new day is written, by whichever process
writes it, and from serve's slow tick at most every 30 minutes. A day file that passes
20 MB SHALL be rotated within the day with one `log.runaway` entry naming the component
and event that produced most of it; past 50 MB in a day, `debug` entries SHALL be dropped
for the rest of the day. Every deletion SHALL be logged as an `act.cleanup` entry.

#### Scenario: A Mac where serve never runs

- **WHEN** serve is not running and a command writes the first entry of a new day
- **THEN** day files older than the retention window are deleted before that entry is
  written, and an `act.cleanup` entry records what was removed

#### Scenario: A loop floods the log

- **WHEN** one component writes 25 MB of entries in a day
- **THEN** that day's file is rotated at 20 MB and a `log.runaway` entry names the
  component and the event responsible

### Requirement: gtmux logs reads the store

`gtmux logs` SHALL show the last hour of entries across components in time order, one
line each, and SHALL accept `--since`, `--until`, `--component`, `--level`, `--acts`,
`--actor`, `--event`, `--follow` (continuing across midnight) and `--json` (the raw
entries).

#### Scenario: What did the phone do today

- **WHEN** the user runs `gtmux logs --since 1d --acts --actor phone`
- **THEN** every action a phone took against this Mac today is listed in time order,
  whichever day files and in-day rotations hold them

### Requirement: Status says when it was written and when it goes stale

A status file SHALL carry `component`, `updated`, `staleAfter` (seconds), `pid`,
`version`, `state`, `since` and `detail`, SHALL be written atomically, and SHALL be
rewritten at least every `staleAfter / 2` while its writer runs. A reader SHALL treat a
status older than `staleAfter` as unknown. A surface SHALL learn a component's state from
status and SHALL NOT parse a log for it.

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

### Requirement: gtmux's own files are readable by their owner only

Files gtmux owns under `~/.local/share/gtmux/` and `~/.config/gtmux/` SHALL be created
0600 and directories 0700. On the first start of a version that includes this requirement,
gtmux SHALL narrow the existing tree to those modes once, leaving files it does not own
(an agent's settings, the user's instruction files, LaunchAgent plists) at their existing
mode, and SHALL remove the retired `hq-feed/spool.jsonl`.

#### Scenario: An existing world-readable journal

- **WHEN** a version with this requirement first starts and `events.jsonl` is 0644
- **THEN** `events.jsonl` is 0600 afterwards and its contents are unchanged

### Requirement: serve records what it saw and what it did

serve SHALL log its start and stop, rejected authentication (aggregated to at most one
entry a minute with a count), pairing codes minted, redeemed and rejected (with the
reason, never the code), recovered handler panics and failed pushes; and SHALL record as
actions, with the device, browser or guest link as actor, every pairing, revocation,
send, focus, upload, share change and push registration it performs.

#### Scenario: A scanner hits the tunnel with bad tokens

- **WHEN** 500 requests with unknown tokens arrive within one minute
- **THEN** the store gains at most one `auth.rejected` entry for that minute, carrying the
  count

### Requirement: Nothing leaves the machine unless the user exports it

gtmux SHALL NOT upload logs, status or crash reports anywhere and SHALL NOT include a
third-party telemetry or crash SDK. Entries leave the machine only through an explicit
export (`gtmux doctor --bundle`, or the phone's share action), which SHALL exclude the
journal and user data unless the user opts the journal in.

#### Scenario: A bug report bundle

- **WHEN** the user runs `gtmux doctor --bundle report.tgz`
- **THEN** the archive holds `logs/`, `status/`, the doctor report and version information,
  no `events.jsonl` and no uploads, and the command lists what it included
