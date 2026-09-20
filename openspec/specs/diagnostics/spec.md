# diagnostics Specification

## Purpose

gtmux's local record of itself: one log store on the Mac holding what gtmux saw and what
it did, status files that say what is true right now, and the rules that keep both
bounded, private and free of credentials. It exists because on 2026-09-19 a phone could
not pair and nothing on the Mac recorded why, and because a surface was reading another
component's log text to decide its state. Built by change `diagnostics` (archived
2026-09-20), in three phases: the store and serve's record, the full action trail, and
the phone's buffer with the bug-report export.

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
entries), reading across day files and in-day segments. `--stats` SHALL answer about the
store instead of printing it: its size, how many files it holds and its oldest day, the
retention bounds in force, how many entries the window holds and how many of them are
warnings or errors, and whether extra detail is being recorded. `--stats --json` SHALL
emit that as one object, which is what a surface reads.

#### Scenario: Why a pairing failed

- **WHEN** the user runs `gtmux logs --event 'act.pair' --since 2h`
- **THEN** every pairing attempt in that window is listed, a refused one with its reason

#### Scenario: A surface asking whether today went wrong

- **WHEN** the menu bar runs `gtmux logs --since <today> --stats --json`
- **THEN** it gets the store's size and bounds and today's entry, warning and error counts,
  and shows the problems when there are any

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
session, share change and push registration it performs. A request made with the master
token MAY name its actor in `X-Gtmux-Actor`, and serve SHALL accept only `user`, `hq`,
`agent:%N`, `menubar`, `system` or `update` there; the CLI and the menu bar send it. It
SHALL publish `status/serve.json` on start and every slow tick.

#### Scenario: A scanner hits the tunnel with bad tokens

- **WHEN** 500 requests with unknown tokens arrive within one minute
- **THEN** the store gains at most two `auth.rejected` entries for that minute, one of them
  carrying the count

### Requirement: Every act that changes something is recorded

Every act that changes something on the Mac, whoever starts it, SHALL be recorded as an
action entry with its actor: every command the command table marks as writing, HQ's acts,
the hook's notifications, restore's resumed conversations and the menu bar's. A command's
actor SHALL come from what its process can see: a delegating component's `GTMUX_ACTOR`
from a closed set (the menu bar sets `menubar`), `hq` when it runs at or inside HQ's home,
`agent:%N` when it runs from a pane whose agent holds the turn, else `user`. serve, the
hook and the tunnel client SHALL record what they do on their own as `system`, and a verb
serve performs for a device SHALL name that device, on that request only. The acts the
journal audits for HQ SHALL be written to the journal and the log by one call, the log
keeping lengths, short hashes and ids in place of the text. Every action event SHALL be
listed in one catalog (`diag.Catalog`) with the commands that record it, rendered into
`docs/cli.md`.

#### Scenario: A command marked as writing has no action

- **WHEN** a command in the command table is marked as writing and the catalog lists no
  event it records
- **THEN** the test suite fails, as it does for an event written and not listed, or listed
  and never written

#### Scenario: HQ sends into a worker

- **WHEN** HQ runs `gtmux send %7` from its home
- **THEN** the journal's audit record holds the payload's head for HQ, and the log holds
  one `act.send` entry by `hq` with the length and a short hash, never the text

### Requirement: A refusal is recorded as a refusal, never as a failure

An act that ends in a delivery SHALL take its outcome from one rule: a state beginning
`refused` is `refused`, `failed` is `failed`, and everything else, `landed`, `queued`,
`sent` and `staged` included, is `ok`. A guard that declines to act is gtmux working as
designed, and a queued payload was accepted; recording either as a failure puts an error
in the diagnostics for doing the right thing.

#### Scenario: The draft guard declines a spawn

- **WHEN** `gtmux spawn` delivers into a pane whose input box already holds someone's
  unsubmitted line, and the guard refuses
- **THEN** the act is recorded `refused` with `refused-draft` as its reason, and
  "problems only" shows it as a warning rather than an error

### Requirement: Debug entries are opt-in per component

`debug` entries SHALL be written only for components named by `GTMUX_DEBUG`
(`serve,tunnel`, or `all`) or by `debug` in `config.json`, which reaches processes launchd
starts; `GTMUX_HOOK_DEBUG`, `GTMUX_TUNNEL_DEBUG` and `GTMUXBAR_DEBUG` SHALL keep working
for their components. The hook's and restore's traces SHALL be entries in the store, and
`hook.log` and `restore.log` SHALL be retired; restore's trace SHALL stay always on.
`gtmux config debug [on|off|<components>]` SHALL read and write that setting, `on` meaning
every component, and a change SHALL take effect for each process as it next starts.

#### Scenario: Turning it up without a terminal

- **WHEN** the user turns on "Record extra detail" in the menu bar's Diagnostics section
- **THEN** `debug` in `config.json` becomes `all`, and serve, the tunnel client and the
  hook write debug entries from their next start

#### Scenario: Why a hook did not fire

- **WHEN** `debug` in `config.json` is `hook` and an agent's turn ends
- **THEN** `gtmux logs --component hook --level debug` shows the hook's decision for that
  event

### Requirement: The menu bar writes to the same store

The menu bar app SHALL write its entries in the store's schema to the day's current file,
each in one append of at most 4 KB with credentials redacted, and SHALL mirror each to the
unified log under subsystem `com.gtmux.menubar`. It SHALL record its start and every
notification it shows or does not show, and a test run SHALL never write to the real store.

#### Scenario: A notification the app did not show

- **WHEN** a queued notification is older than 30 seconds when the app reads it
- **THEN** the store gains an `act.notify.post` entry by `menubar`, refused, with reason
  `stale`

### Requirement: Every other store that grows is bounded

launchd's captures of a gtmux daemon's stdout and stderr SHALL be written to
`logs/<component>.stderr` when a plist is written, and disk hygiene SHALL cap them and the
legacy captures in the data root alike. The transcript miner SHALL prune error signatures
unseen for 90 days and the read marks of logs that no longer exist, logging the pruning,
and SHALL keep its pass history to the newest 365 once it passes 730.

#### Scenario: A plist written before the move

- **WHEN** serve still runs from a plist that captures to `serve.log` and that file passes
  its cap
- **THEN** the hygiene sweep trims it to its tail, as it does `logs/serve.stderr`

### Requirement: Nothing is uploaded

gtmux SHALL NOT upload logs, status or crash reports anywhere and SHALL NOT include a
third-party telemetry or crash SDK.

#### Scenario: A week of use

- **WHEN** gtmux runs for a week with serve and the tunnel on
- **THEN** its logs and status exist only under `~/.local/share/gtmux/` on the Mac

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
on the device, and leave it only from the diagnostic record page, whose Copy and Share
hand over a header line and the entries as JSON lines.

#### Scenario: A dead Mac does not push out the pairing that explains it

- **WHEN** the Mac stops answering and the app polls it every few seconds for ten minutes
- **THEN** the buffer gains about one failure entry a minute per route, and the pairing
  attempt recorded before it is still there

### Requirement: Each surface shows what it recorded, in words

A person reading a diagnostic record is already having a bad day, so each surface SHALL
show its own record rather than only offer to hand it over, and SHALL show each entry as a
sentence with its raw attributes underneath it.

The menu bar SHALL carry a Diagnostics section: how much the store holds, how long it is
kept and how many of today's entries are warnings or errors; a window showing the last
three days, newest first, filtered to everything or to problems only; a one-click bug
report (`gtmux doctor --bundle`) that names where the file landed; and a switch for
recording extra detail that says it applies to each process as it starts.

The phone and iPad app SHALL carry a diagnostic record page reached from one settings row,
that row stating the number of problems when there are any and how much is kept when there
are none. The page SHALL group entries by day with today and yesterday named, mark a
warning or an error, filter to problems only, and offer Copy, Share and Clear. Before
anything is recorded it SHALL say what will show up there and that credentials are
replaced before anything is written down.

Entry text SHALL be written for the reader of that surface: the app translates its own
entries, and the Mac's store stays English because its entries are what a bug report
quotes.

#### Scenario: A phone that stopped updating

- **WHEN** the Mac stops answering and the person opens the diagnostic record
- **THEN** the page says the Mac could not be reached, on which route, after how long and
  how many times it repeated, without their having to read `api.failed` or `status=0`

#### Scenario: Nothing has gone wrong yet

- **WHEN** the record is empty
- **THEN** the page says what shows up there and that nothing is uploaded, instead of
  showing an empty list
