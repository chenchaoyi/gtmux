# diagnostics Specification

## Purpose

gtmux's local record of itself: one log store on the Mac holding what gtmux saw and what
it did, status files that say what is true right now, and the rules that keep both
bounded, private and free of credentials. It exists because on 2026-09-19 a phone could
not pair and nothing on the Mac recorded why, and because a surface was reading another
component's log text to decide its state. Built by change `diagnostics` (phase 1); the
full action trail, the phone's buffer and the bug-report export are still in flight there.

## Requirements

### Requirement: One local log store for diagnostics and actions

Every gtmux process SHALL write its log entries to one store under
`~/.local/share/gtmux/logs/`, one file per local day. An entry SHALL be one JSON object
with `ts` (RFC 3339, milliseconds, local offset), `level` (`debug`, `info`, `warn`,
`error`), `component`, `kind` (`diag` or `act`), `event` (a stable dotted name; action
events start with `act.`), `msg` and `attrs` (a flat map of scalars). An action entry SHALL
also carry `actor`, `target` and `outcome` (`ok`, `refused` with a reason, or `failed` with
an error). Entries SHALL be in English. Each entry SHALL be appended in a single write of
at most 4 KB, and a failure to log, including a panic in the logging path, SHALL NOT fail
the operation that was logging.

#### Scenario: Two processes log at once

- **WHEN** serve and a hook append entries to the same day file at the same moment
- **THEN** both entries are present as whole lines and neither is interleaved with the
  other

### Requirement: Entries carry no user content and no credentials

An entry SHALL NOT carry prompt or message text, pane screens, upload file names or
knowledge entry bodies; a send SHALL record its length and a short hash of its payload.
The writer SHALL replace, in every entry: every secret registered with it (the serve
token, relay token, Direct secret, device and guest tokens as issued); the value of any
attribute whose key names a credential; pairing and share URL fragments; and
`Authorization` values, scheme included.

#### Scenario: A call site logs a token by mistake

- **WHEN** a component logs a message or attribute that contains the serve token
- **THEN** the written entry holds a redaction marker in its place and the token does not
  appear in the store

### Requirement: The log store bounds its own growth

The store SHALL delete entries older than 30 days and, while it is over 100 MB, SHALL
delete the oldest days first, never today's file; both limits SHALL be configurable
(`logs.retainDays`, `logs.maxMB`). The cleanup SHALL run when the first entry of a new day
is written, by whichever process writes it, and from serve's hygiene sweep. A day file
that passes 20 MB SHALL be rotated within the day with one `log.runaway` entry naming the
component and event that produced most of it; past 50 MB in a day, `debug` entries SHALL
be dropped for the rest of the day. Every deletion SHALL be logged as an `act.cleanup`
entry.

#### Scenario: A Mac where serve never runs

- **WHEN** serve is not running and a command writes the first entry of a new day
- **THEN** day files older than the retention window are deleted and an `act.cleanup`
  entry records what was removed

### Requirement: gtmux logs reads the store

`gtmux logs` SHALL show the last hour of entries across components in time order, one
line each, and SHALL accept `--since`, `--until`, `--component`, `--level`, `--acts`,
`--actor`, `--event`, `--follow` (continuing across midnight) and `--json` (the raw
entries), reading across day files and in-day segments.

#### Scenario: Why a pairing failed

- **WHEN** the user runs `gtmux logs --event 'act.pair' --since 2h`
- **THEN** every pairing attempt in that window is listed, a refused one with its reason

### Requirement: Status says when it was written and when it goes stale

A status file SHALL carry `component`, `updated`, `staleAfter` (seconds), `pid`,
`version`, `state`, `since` and `detail`, SHALL be written atomically, and SHALL be
rewritten at least every `staleAfter / 2` while its writer runs; `since` SHALL carry over
while the state is unchanged. A reader SHALL treat a status older than `staleAfter` as
unknown. A surface SHALL learn a component's state from status and SHALL NOT parse a log
for it; `check-design.sh` fails a menu bar source that names a `.log` file, outside one
written-down exception that reads a log only to show its last line.

#### Scenario: The writer died

- **WHEN** the tunnel client exits without cleaning up and its status is 5 minutes old with
  `staleAfter` 90
- **THEN** readers report the tunnel's state as unknown, not as the last state written

### Requirement: Tunnel status for both backends

`status/tunnel.json` SHALL report the active backend and one of `connecting`, `connected`
or `down`, with the last error. Under Direct the tunnel client SHALL write it from a probe
of its own pairing URL, every 5 seconds until the first success and every 30 seconds after,
and SHALL remove it on a graceful stop. Under Standard serve SHALL write it from
cloudflared's `cloudflared_tunnel_ha_connections` metric, read from the metrics address
gtmux passes to cloudflared explicitly. A tunnel SHALL read `down` after a failure once
connected, or after a minute of connecting without success, and only those transitions
SHALL be logged.

#### Scenario: Direct cannot resolve its server

- **WHEN** the Mac's network cannot resolve the Direct server's name for over a minute
- **THEN** `status/tunnel.json` reports backend `direct`, state `down`, and the resolver's
  error as the last error, and one `tunnel.disconnected` entry is logged

### Requirement: gtmux's own files are readable by their owner only

Every gtmux entry point, the CLI and the menu bar app, SHALL set a `077` umask before it
creates a file. Files gtmux edits but does not own (an agent's settings, the user's
instruction files, `~/.tmux.conf`, a repository's `AGENTS.md`, a Warp launch config) SHALL
keep their mode when rewritten and get the conventional mode when created. serve SHALL
narrow `~/.local/share/gtmux/` and `~/.config/gtmux/` on start and on every hygiene sweep,
removing group and other permissions and keeping the owner's, and skipping what it cannot
change; the changes are logged as `act.narrow`.

#### Scenario: An existing world-readable journal

- **WHEN** serve starts and `events.jsonl` is 0644
- **THEN** `events.jsonl` is 0600 afterwards and its contents are unchanged

### Requirement: Every gtmux path is built in one place

Every path under `~/.config/gtmux/` or `~/.local/share/gtmux/` SHALL be built through
`internal/state` in the CLI and through `Paths.swift` in the menu bar app;
`check-design.sh` fails a hand-joined root anywhere else. `~/.config/gtmux/` holds what a
person set or would carry to a new machine; `~/.local/share/gtmux/` holds what gtmux
generates, with `logs/`, `status/` and `cache/`. The tunnel's recorded address lives in the
data root, and readers fall back to its old place in the config root for one release.

#### Scenario: A file moves between the roots

- **WHEN** a gtmux file moves from one root to the other
- **THEN** the change is one edit in `internal/state` and one in `Paths.swift`, and the
  build fails on any other place that still names the old root

### Requirement: serve records what it saw and what it did

serve SHALL log its start and stop, rejected authentication (the first in a minute with
its route, the rest of that minute as one count), and recovered handler panics; and SHALL
record as actions, with the device, browser or guest link behind the token (or `owner` for
the master token) as actor, every pairing (a refusal naming `expired`, `used`, or
`unknown` to this boot), code minted, revocation, send, focus, upload, remote terminal
session, share change and push registration it performs. It SHALL publish
`status/serve.json` on start and every slow tick.

#### Scenario: A scanner hits the tunnel with bad tokens

- **WHEN** 500 requests with unknown tokens arrive within one minute
- **THEN** the store gains at most two `auth.rejected` entries for that minute, one of them
  carrying the count

### Requirement: Nothing is uploaded

gtmux SHALL NOT upload logs, status or crash reports anywhere and SHALL NOT include a
third-party telemetry or crash SDK.

#### Scenario: A week of use

- **WHEN** gtmux runs for a week with serve and the tunnel on
- **THEN** its logs and status exist only under `~/.local/share/gtmux/` on the Mac
