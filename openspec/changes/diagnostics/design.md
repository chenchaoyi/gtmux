# diagnostics — design

## 1. What the audit found

Measured on 2026-09-19 on the machine that reported the pairing failure, against v1.0.33.
Counts are from that machine; code references are to `main` at the time.

### 1.1 The one question nobody could answer

A phone scanned a pairing QR and could not pair. The phone knew why (it classifies the
failure), but it keeps no record. The serve that received the request writes nothing at
runtime: across `internal/server` and `internal/app/serve*.go` there are zero log calls,
so a rejected pairing code, a rejected token and a failed send all vanish. The menu bar
app keeps no persistent log either; it has a debug switch that prints to stderr, which
goes nowhere when it is started as a login item. The diagnosis had to be rebuilt from
code reading and fresh probes, and the phone's exact message is still unknown.

### 1.2 Components that fail silently

| Component | What it records today | What goes unrecorded |
|---|---|---|
| serve | a startup banner, into the launchd capture | every request outcome: auth failures, pairing redeemed or rejected, device revoked, send failures |
| Direct tunnel client | nothing (chisel's own output is switched off; `selftunnel.log` last written 2026-07-11) | connect, disconnect, the reason it cannot connect |
| Standard tunnel (cloudflared) | its own text log, `tunnel.log` | fine, but gtmux reads its state by matching three phrases in that text |
| hook | nothing unless `GTMUX_HOOK_DEBUG` is set | the reason a hook did not update a pane |
| menu bar app | nothing unless launched by hand with `GTMUXBAR_DEBUG=1` | failed CLI calls, failed code mints, reachability verdicts |
| phone and iPad | one `console.log` in the whole app, no crash reporting | pairing attempts, connection changes, push registration |
| restore | `restore.log`, text, truncated to zero past 512 KB | fine for its purpose |

### 1.3 State read by scraping text

- The pairing window decides whether "the tunnel is down" by reading the last 16 KB of
  cloudflared's `tunnel.log` and matching three phrases. It does this for either backend,
  so a Direct user's verdict comes from a log of a tunnel they do not run. On the audited
  machine that tail held neither a registration nor any of the three phrases (the most
  recent failure read "Tunnel not found", which none of them match), so the verdict was
  "not down" regardless of the truth.
- cloudflared publishes the real number on its metrics endpoint
  (`cloudflared_tunnel_ha_connections` on 127.0.0.1:20241 on that machine). Nothing in
  gtmux reads it.
- `gtmux doctor`'s remote-access section checks that cloudflared is installed (the
  audited machine uses Direct), that serve answers locally, and reports "the phone can
  reach this Mac" on the strength of the second. Whether the tunnel is connected is not
  checked at all.

### 1.4 Permissions and contents

Everything under `~/.local/share/gtmux/` is created 0644 under 0755 directories, in a home
directory that is itself 0755, so every account on the Mac can read it. 88 MB on the
audited machine, including:

- `serve.log`: the master token 398 times and 395 one-time pairing links (fixed in
  v1.0.33 for new writes and for the file's mode; the rest of this list is not).
- `events.jsonl`: 56,067 records since 2026-07-13, 16 MB; `summary` carries text from the
  agents' conversations.
- `mine/errors.json`: raw error lines lifted from agent transcripts, ANSI codes and all.
- `uploads/`: images sent from the phone.
- `octrans/`: gtmux's own copy of opencode transcripts.

The token file itself is 0600. The files around it undid that protection.

### 1.5 Leftovers and bounds

- `hq-feed/spool.jsonl`, 5.5 MB, last written 2026-08-18. Change
  `retire-perception-spool` removed the layer; nothing removed the file.
- The launchd captures (`serve.log`, `tunnel.log`, `selftunnel.log`, `restore.log`) are
  trimmed by `diskhygiene` from 8 MB to their last 2 MB. `events.jsonl` rotates at 20 MB
  with one generation. `hook.log` and `mine/` have no bound. Three mechanisms, three sizes.

### 1.6 Actions leave no trail

The journal has an audit sub-namespace (`gtmux:audit:*`) for seven acts HQ needs to see:
wake delivered, wake dropped, `gtmux send`, reap, rotate, the HQ session changing, and a
knowledge mutation. Every other act that changes something leaves nothing behind: a
focus, a phone's send or upload, a device paired or revoked, a share link created, hooks
installed into an agent's settings, an update, each change `doctor --fix` applied, a
config value set, files deleted by disk hygiene. "What did gtmux do to this Mac last
Tuesday, and on whose behalf" has no answer.

### 1.7 What already works and is kept

`events.jsonl` is a proper structured stream: one JSON object per line, a sequence number,
a severity, rotation by rename so a follower never loses its place, and a documented
reader (`gtmux events`). It is the product's event stream for HQ and stays exactly that.
This design borrows its shape for logs and does not merge the two.

## 2. The model: one local log, like the system log

gtmux gets one log store on the Mac, in the spirit of the macOS unified log: every gtmux
process writes to it, it holds both diagnostics and a trail of actions, it is English, it
is bounded and cleaned by itself, and one tool reads it.

| macOS | gtmux |
|---|---|
| one store for every process | `~/.local/share/gtmux/logs/`, written by serve, the tunnel client, the hook, every command, the menu bar app |
| subsystem and category | `component` and `event` |
| levels (default, info, debug, error, fault) | `info`, `debug`, `warn`, `error` |
| dynamic values `<private>` unless marked public | no user content by rule; credentials replaced where the line is written |
| `log show --last 1h --predicate …` | `gtmux logs --since 1h --component … --event …` |
| `log stream` | `gtmux logs --follow` |
| Console.app | the menu bar app also mirrors its own entries to the unified log, so they show in Console.app |
| retention managed by the system | 30 days or 100 MB, whichever comes first, cleaned by gtmux itself (section 5) |
| `log collect` into a `.logarchive` | `gtmux doctor --bundle` |

Three other kinds of file stay separate, because they have different readers:

- **Status** answers "what is true right now", one small JSON file per component,
  overwritten. Surfaces read status and never parse the log for state (section 7).
- **The journal**, `events.jsonl`, stays HQ's event stream, unchanged apart from its mode.
- **User data** (uploads, transcript copies, the mining ledger) stays with its feature.

The Go side cannot write to the unified log directly: it would need cgo, and the CLI
must stay cgo-free. Shelling out to `logger` for every line is too slow for the hook. So
the store is files, and `gtmux logs` plays the part of `log show`.

## 3. Layout and permissions

```
~/.local/share/gtmux/              0700
  logs/2026-09-19.jsonl            0600   one file per local day, every component
  logs/2026-09-19.1.jsonl          0600   only if a day passes the in-day cap (5.2)
  logs/<component>.stderr          0600   launchd capture: panics and early startup only
  status/<component>.json          0600   overwritten atomically
  events.jsonl                     0600   the journal, unchanged apart from its mode
  uploads/ octrans/ mine/ …        0700 / 0600
```

Every file gtmux owns under its data and config directories is 0600 and every directory
0700. Files gtmux edits that belong to something else (an agent's settings file, the
user's `~/.claude/CLAUDE.md`, a LaunchAgent plist) keep their existing mode.

A one-time migration on the first start of the new version narrows the existing tree,
removes `hq-feed/spool.jsonl`, and writes a marker so it does not run again.

## 4. Entries

### 4.1 Schema

A diagnostic entry:

```json
{"ts":"2026-09-19T09:36:05.123+08:00","level":"warn","component":"serve","kind":"diag",
 "event":"enroll.rejected","msg":"pairing code rejected","attrs":{"reason":"expired","via":"tunnel"}}
```

An action entry adds who did it, to what, and how it ended:

```json
{"ts":"2026-09-19T09:41:12.004+08:00","level":"info","component":"serve","kind":"act",
 "event":"act.send","actor":"phone:3f9c20e1","target":"%7","outcome":"ok",
 "msg":"typed into a pane","attrs":{"bytes":42,"sha":"a1b2c3d4","via":"tunnel","ms":180}}
```

- `ts`: RFC 3339 with milliseconds and the local offset.
- `level`: `debug`, `info`, `warn`, `error`. Actions are `info` when they succeed and
  `warn` or `error` when they are refused or fail.
- `component`: `serve`, `tunnel`, `hook`, `cli`, `hq`, `restore`, `hygiene`, `menubar`.
- `kind`: `diag` or `act`.
- `event`: a stable dotted name, the contract that tests and bug reports refer to. Action
  events start with `act.`. `msg` may be reworded freely.
- `actor` (actions only): `user` (a command typed in a terminal), `agent:%N` (a gtmux
  command run from another agent's pane), `hq`, `menubar`, `phone:<device>`,
  `browser:<device>`, `guest:<link>`, `system` (serve's ticks, the hook), `update`.
  Attribution uses what gtmux already knows (the HQ home, `$TMUX_PANE`, the device token
  that authenticated the request) and records what it saw, not a guess.
- `target`: what was acted on (a pane id, a session, a device id, a file gtmux owns).
- `outcome` (actions only): `ok`, `refused` (with `attrs.reason`, for example
  `refused-draft`), or `failed` (with `attrs.error`).
- `attrs`: a flat map of scalars.

English only. The event name is the stable part; `gtmux logs` renders its own chrome in
both languages.

### 4.2 What an entry never carries

No prompt or message text, no pane screen, no file names from uploads, no knowledge entry
bodies. A send records its length and a short hash of the payload, so it can be matched
to the journal's receipt without keeping the text. This is the rule the unified log
enforces with `<private>`, made absolute because nothing here needs the content.

Credentials are replaced where the line is written, not at each call site: values
registered at startup (the serve token, relay token, Direct secret, device tokens as they
are issued), any attr whose key names a credential (`token`, `secret`, `code`, `auth`,
`password`), pairing and share fragments (`#c=…`, `#g=…`) and `Authorization` values. A
test logs every registered secret in every position and asserts none survives.

### 4.3 Writing safely from many processes

Every gtmux process appends to the same day file. Each entry is one `write` of at most 4
KB with `O_APPEND`, which appends atomically on a local file system; `events.jsonl`
already relies on this across concurrent hooks. A longer entry is cut and marked. A
failure to log never fails the command that was logging.

### 4.4 Debug

`info` and above are always written. `GTMUX_DEBUG=serve,tunnel` (or `all`), or `debug` in
`~/.config/gtmux/config.json`, adds `debug` entries for the named components.
`GTMUX_HOOK_DEBUG`, `GTMUX_TUNNEL_DEBUG` and `GTMUXBAR_DEBUG` stay as aliases.

## 5. Growth: rotation and cleanup

### 5.1 The log store

- A new file starts each local day, so rotation needs no rename and a follower (`gtmux
  logs --follow`) simply moves to the next file at midnight.
- Retention: entries older than 30 days are deleted, and while `logs/` is over 100 MB the
  oldest days are deleted first. Today's file is never deleted. Both limits are settable
  (`logs.retainDays`, `logs.maxMB` in `config.json`).
- Cleanup does not depend on serve. The first entry written on a new day, by whichever
  process writes it, runs the cleanup once (guarded by a marker so two processes do not
  both run it); serve's slow tick runs it too, at most every 30 minutes.

Expected volume: an action is about 250 bytes and a busy day has a few thousand, with a
few hundred diagnostic lines, so under 1 MB a day and around 30 MB for the retention
window. The cap is there for the day something loops.

### 5.2 A runaway writer

A day file that passes 20 MB is rotated within the day (`2026-09-19.1.jsonl`) and one
`log.runaway` entry names the component and event that produced most of it, so doctor can
point at the loop instead of only reporting size. Past 50 MB in a day, `debug` entries
are dropped for the rest of that day and the drop is recorded.

### 5.3 Every other thing that grows

One table, one owner each, so nothing grows unbounded again:

| Store | Bound | Enforced by |
|---|---|---|
| `logs/` | 30 days / 100 MB, 20 MB in-day guard | any writer on a new day, serve's slow tick |
| `events.jsonl` | 20 MB, one older generation (unchanged) | the journal writer |
| launchd captures | 8 MB trimmed to 2 MB (unchanged) | disk hygiene |
| `uploads/` | 7 days / 200 MB (unchanged) | disk hygiene |
| `mine/` | signatures unseen for 90 days pruned, `passes.jsonl` rotated | the mining pass |
| `hq-feed/spool.jsonl` | removed | the migration |

Every deletion by cleanup or hygiene is itself an action entry (`act.cleanup`, with what
was removed and how many bytes), so space never disappears without a trace.

## 6. The action trail

The trail covers every act that changes something, whoever started it:

| Area | Actions |
|---|---|
| panes | focus, send (text or keys), upload into a pane, spawn, reap, adopt, new session, restore (tabs opened, conversations resumed), attach opened and closed |
| access | pairing code minted, device enrolled, device revoked, push token registered or forgotten, share link created, changed or revoked, input consent changed, tunnel on, off or backend changed, LAN serve on or off, awake on or off |
| HQ | wake delivered or dropped, rotate, knowledge changed, capture, quiet changed |
| setup | hooks installed or removed (which agent, which file), the app installed, update (from, to), each change `doctor --fix` applied, a config key set (credential values redacted) |
| housekeeping | disk hygiene deletions, log cleanup, the permission migration |
| menu bar | notification posted, update started from the menu |

The seven acts the journal already audits keep going to the journal, because HQ reads
them there. One call writes both, so the two records of the same act cannot disagree and
the journal stays HQ's stream rather than the whole trail.

The command table's `writes` flag is the checklist: a test fails when a command marked as
writing has no action event in the catalog, the same way the help test fails a dispatched
command missing from the table.

## 7. Status

### 7.1 Schema

```json
{"component":"tunnel","updated":"2026-09-19T09:36:07+08:00","staleAfter":90,
 "pid":61144,"version":"1.0.34","state":"connected","since":"2026-09-19T09:36:06+08:00",
 "detail":{"backend":"direct","server":"tunnel.ccy.dev","lastError":"","lastErrorAt":""}}
```

`updated` and `staleAfter` let a reader tell "the writer died" from "the state is X": a
status older than `staleAfter` seconds reads as unknown. A writer rewrites its file at
least every `staleAfter / 2`. Writes are atomic (temp file, then rename).

### 7.2 Tunnel status, both backends

- Direct: the tunnel client writes it from the chisel connection and from a probe of its
  own pairing URL every 30 seconds, the path the phone takes. The probe resolves the name
  the client dials, so a DNS failure shows up as `down` with the resolver's error.
- Standard: gtmux starts cloudflared with an explicit `--metrics 127.0.0.1:<port>`, and
  serve's slow tick reads `cloudflared_tunnel_ha_connections`. Above zero is connected. No
  log text is read.

A transition between connected and down is also a log entry, and a journal control record
so HQ hears that the phone can no longer reach the Mac.

### 7.3 Readers

- The pairing window, when its own probe fails, reads `status/tunnel.json`: connected and
  fresh means this Mac cannot see its own address but the tunnel is up; down means no
  device connects, with the recorded error; stale or missing means it cannot say. The
  cloudflared phrase match is removed.
- Serve publishes `status/serve.json` (boot, port, version, backend).

## 8. Doctor

A new `Logs` section, next to the existing disk rows:

| Row | OK when | Flagged when | `--fix` |
|---|---|---|---|
| log store | under its cap, oldest day within retention | over the cap, or files older than retention + 2 days exist (cleanup is not running) | run cleanup now |
| runaway | no in-day rotation in the last 7 days | a day passed 20 MB; names the component and event | none; it points at the loop |
| recent errors | no `error` entries in 24 hours | counts by component and event, most frequent first | none |
| file modes | every gtmux-owned file 0600, directory 0700 | anything wider, with the count and an example path | narrow them |
| other stores | each within its bound in section 5.3 | the store and its size | the owner's cleanup, or removal of a retired file |

The remote-access section gains a tunnel row from `status/tunnel.json` (backend, state,
since, last error). The cloudflared row applies only to Standard, and "the phone can reach
this Mac" is said only when the status shows it.

## 9. Reading and leaving the machine

`gtmux logs` is to this store what `log show` and `log stream` are to the system's. Without
it, reading means knowing the file layout, stitching day files and in-day rotations
together, and reading raw JSON; with it:

- `gtmux logs` shows the last hour, merged across components, one line each:
  `09:41:12 serve  act.send  phone:3f9c20e1 → %7 ok · 42 bytes via tunnel`.
- `--since 2d`, `--until`, `--component serve`, `--level warn`, `--acts` (only actions),
  `--actor phone`, `--event 'enroll.*'` narrow it.
- `--follow` streams new entries across midnight.
- `--json` prints the raw entries, so HQ or a coding agent diagnosing a problem reads one
  stable interface instead of file paths.

The phone and iPad cannot write to the Mac's store. They keep their own buffer of the last
500 entries (200 KB on disk), in the same schema and under the same redaction, under
Settings → Diagnostics with copy and share.

`gtmux doctor --bundle <path>` is the `log collect`: an archive of `logs/`, `status/`, the
doctor report and versions, without the journal or user data unless `--with-events` is
given, listing what it packed.

Nothing is uploaded anywhere. There is no telemetry and no third-party crash SDK.

## 10. Enforcement

- Tests: redaction (4.2), the in-day guard and retention (5), one per serve and tunnel
  event, and the command table's `writes` flag against the action catalog (6).
- `check-design.sh` fails a literal `0o644` / `0o755` (and `0644` / `0755`) mode in
  `internal/` outside an allowlist of files gtmux does not own; writers go through `state`
  helpers that apply 0600 / 0700.
- `check-design.sh` fails a surface that reads `tunnel.log` or any `*.log` for state.

## 11. Phasing

1. **The store and the incident.** The logger, redaction, retention and cleanup, the
   permission migration, `gtmux logs`, doctor's `Logs` section, serve's diagnostics and
   its actions (pairing, revoke, send, focus, upload, share, push), tunnel status for both
   backends with the pairing window and doctor reading it.
2. **The full trail.** Actions for every command marked as writing, HQ and housekeeping
   acts, the journal's audit written through the same call, the menu bar writing to the
   store, hook and restore moved onto it, the single debug switch, launchd captures under
   `logs/`, `mine/` bounded.
3. **Leaving the machine.** The phone and iPad diagnostics buffer, `gtmux doctor --bundle`.

Each phase ships on its own and leaves the system consistent.

## 12. Decisions

- One store for every component, as the system log does, rather than a file per
  component: a question like "what happened around 09:36" spans serve, the tunnel and the
  menu bar, and one time-ordered store answers it without a merge.
- Diagnostics and actions share the store and the schema, told apart by `kind`. The
  journal stays separate: HQ reads it, and filling it with every act and every diagnostic
  would bury what HQ needs.
- English only, not localized.
- The token is not rotated. After the migration the old copies in `serve.log` are readable
  by the owner only, the same as the token file.
- Permissions are fixed by migration, not only for new files: the exposure in 1.4 is in
  files that already exist.
- Files rather than the unified log for the Go side, because the CLI stays cgo-free. The
  menu bar mirrors its own entries to the unified log in addition.
