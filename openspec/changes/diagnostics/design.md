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

### 1.6 What already works and is kept

`events.jsonl` is a proper structured stream: one JSON object per line, a sequence number,
a severity, rotation by rename so a follower never loses its place, and a documented
reader (`gtmux events`). It is the product's event stream for HQ and stays exactly that.
This design borrows its shape for logs and does not merge the two.

## 2. Four kinds of record

Every file gtmux writes about itself is one of four kinds. They have different readers,
lifetimes and privacy, and mixing them is what produced most of section 1.

| Kind | Answers | Reader | Shape | Lifetime |
|---|---|---|---|---|
| status | what is true right now | surfaces, doctor | one small JSON object per component, overwritten | current only |
| log | what happened, for someone debugging | a person, `gtmux logs`, a bug report | JSON Lines, one schema across components | bounded by size |
| journal | what happened, for HQ | HQ, `gtmux events` | `events.jsonl` as today | bounded by size |
| user data | things the user made or sent | the features that use them | as today | per feature |

Rules that follow from the split:

1. A surface never parses a log to learn state. If it needs to know something, a
   component publishes it as status.
2. A log never carries user content: no prompt text, no message text, no pane screen, no
   upload names. It carries event names, ids, reasons and counts.
3. A state change that alters what the user can do is also written to the journal as a
   control record, so HQ hears about it (first case: the tunnel going down or coming back).

## 3. Layout and permissions

```
~/.local/share/gtmux/          0700
  status/<component>.json      0600   overwritten atomically
  logs/<component>.jsonl       0600   + .1 .2 generations
  logs/<component>.stderr      0600   launchd capture: panics and early startup only
  events.jsonl                 0600   journal, unchanged apart from its mode
  uploads/ octrans/ mine/ …    0700/0600
```

Every file gtmux owns under its data and config directories is 0600 and every directory
0700. Files gtmux edits that belong to something else (an agent's settings file, the
user's `~/.claude/CLAUDE.md`, a LaunchAgent plist) keep their existing mode.

A one-time migration on the first start of the new version narrows the existing tree,
removes `hq-feed/spool.jsonl`, and writes a marker so it does not run again. The launchd
captures move under `logs/` when the plists are next regenerated; until then the old paths
keep working.

## 4. Logs

### 4.1 Schema

```json
{"ts":"2026-09-19T09:36:05.123+08:00","level":"warn","component":"serve",
 "event":"enroll.rejected","msg":"pairing code rejected","attrs":{"reason":"expired","via":"tunnel"}}
```

- `ts`: RFC 3339 with milliseconds and the local offset. Logs are read by people.
- `level`: `debug`, `info`, `warn`, `error`.
- `component`: `serve`, `tunnel`, `hook`, `restore`, `hq`, `cli`, `menubar`.
- `event`: a stable dotted name. It is the contract: tests assert on it and a bug report
  can be searched for it. `msg` may be reworded freely.
- `attrs`: a flat map of scalars.

Logs are English and are not localized, like `docs/TROUBLESHOOTING.md`: they are
maintainer material. `gtmux logs` renders its own chrome in both languages.

### 4.2 Redaction at the sink

The logger, not each call site, keeps secrets out:

- values registered at startup (the serve token, relay token, Direct secret, device
  tokens as they are issued) are replaced wherever they appear;
- an attr whose key names a credential (`token`, `secret`, `code`, `auth`, `password`) is
  replaced;
- pairing and share fragments (`#c=…`, `#g=…`) and `Authorization:` values are replaced.

A test logs every registered secret in every position and asserts none survives.

### 4.3 Levels and bounds

`info` and above are written by default. Default volume is low by construction: lifecycle
transitions, rejections and failures, not every request. A flood source (for example a
scanner hitting the tunnel with bad tokens) is aggregated: `auth.rejected` is written at
most once a minute with a count.

Each component's file rotates by rename at 2 MB and keeps two older generations, the same
mechanism `events.jsonl` uses, so a follower never loses its place and no file is
truncated in place. Worst case is 6 MB per component.

### 4.4 Debug

One switch replaces three: `GTMUX_DEBUG=serve,tunnel` (or `all`), also settable as
`debug` in `~/.config/gtmux/config.json`. It lowers the named components to `debug`.
`GTMUX_HOOK_DEBUG`, `GTMUX_TUNNEL_DEBUG` and `GTMUXBAR_DEBUG` stay as aliases.

## 5. Status

### 5.1 Schema

```json
{"component":"tunnel","updated":"2026-09-19T09:36:07+08:00","staleAfter":90,
 "pid":61144,"version":"1.0.34","state":"connected","since":"2026-09-19T09:36:06+08:00",
 "detail":{"backend":"direct","server":"tunnel.ccy.dev","lastError":"","lastErrorAt":""}}
```

`updated` and `staleAfter` let a reader tell "the writer died" from "the state is X": a
status older than `staleAfter` seconds reads as unknown. A writer rewrites its file at
least every `staleAfter / 2`. Writes are atomic (temp file, then rename).

### 5.2 Tunnel status, both backends

- Direct: the tunnel client writes it. It reports the chisel connection and, every 30
  seconds, an end-to-end probe of its own pairing URL, the path the phone takes. The probe
  resolves the same name the client dials, so a DNS failure shows up as "down" with the
  resolver's error, which is the truth for Direct.
- Standard: gtmux starts cloudflared with an explicit `--metrics 127.0.0.1:<port>`, and
  serve's slow tick reads `cloudflared_tunnel_ha_connections` from it. Above zero is
  connected. No log text is read.

### 5.3 Readers

- The pairing window, when its own probe fails, reads `status/tunnel.json`: connected and
  fresh means this Mac cannot see its own address but the tunnel is up (a phone on
  cellular connects); down means no device connects, with the recorded error; stale or
  missing means it cannot say. The cloudflared text match is removed.
- `gtmux doctor` gains a tunnel row (backend, state, since, last error). The cloudflared
  row applies only to Standard. "The phone can reach this Mac" is said only when that is
  what the status shows.
- Serve publishes `status/serve.json` (boot, port, version, backend).

## 6. What each component records

| Component | status | log events (default level) |
|---|---|---|
| serve | serve.json | `serve.start`, `serve.stop`, `auth.rejected` (aggregated), `enroll.minted`, `enroll.redeemed`, `enroll.rejected`, `device.revoked`, `send.failed`, `push.failed`, `handler.panic` |
| tunnel (Direct) | tunnel.json | `tunnel.connected`, `tunnel.disconnected`, `tunnel.probe.failed` |
| tunnel (Standard) | tunnel.json, written by serve | `tunnel.connected`, `tunnel.disconnected` |
| hook | none | `hook.error` (cannot write state, cannot resolve pane); debug trace behind the switch |
| restore | none | `restore.*`, moved from the text file |
| cli | none | `attach.failed`, `update.failed`, and other command failures worth keeping |
| menu bar | none | `cli.failed`, `mint.failed`, `reach.verdict`, `update.failed` |
| phone, iPad | none | `pair.attempt`, `pair.failed` (with the classified cause and HTTP status), `connection.changed`, `push.register` |

## 7. Reading and leaving the machine

- `gtmux logs [component…] [--level warn] [--since 1h] [--follow] [--json]` merges the
  files by time: `09:36:05 serve warn enroll.rejected  pairing code rejected · reason=expired via=tunnel`.
- The menu bar app writes `logs/menubar.jsonl` in the same schema, so `gtmux logs` shows
  it with the rest, and mirrors to the unified logging system for Console.app.
- The phone and iPad keep a ring buffer of the last 500 entries (capped at 200 KB on
  disk), redacted the same way, under Settings → Diagnostics, with copy and share.
- `gtmux doctor --bundle <path>` writes a tarball of `logs/`, `status/`, the doctor report
  and version information for a bug report. The journal and user data are excluded;
  `--with-events` opts the journal in. It lists what it included.

Nothing is uploaded anywhere. There is no telemetry and no third-party crash SDK. A log
leaves the machine only when the user exports it.

## 8. Enforcement

- A test for redaction (4.2) and one per default event in section 6 for serve and the
  tunnel, so "why did the scan fail" stays answerable.
- `check-design.sh` fails a literal `0o644` / `0o755` (and `0644` / `0755`) file or
  directory mode in `internal/` outside an allowlist of files gtmux does not own. Writers
  go through `state` helpers that apply 0600 / 0700.
- `check-design.sh` fails a surface that reads `tunnel.log` or any `*.log` for state.

## 9. Phasing

1. Foundations and the Direct misdiagnosis: the logger, status files, the permission
   migration, serve's events, tunnel status for both backends, the pairing window and
   doctor reading status.
2. Reading: `gtmux logs`, the single debug switch, hook and restore on the new logger,
   launchd captures under `logs/`, the menu bar's log, tunnel transitions in the journal.
3. Leaving the machine: the phone and iPad diagnostics buffer, `gtmux doctor --bundle`.

Each phase ships on its own and leaves the system consistent.

## 10. Decisions

- Logs and the journal stay separate. HQ reads the journal; a log is for someone
  debugging. Merging them would put debug noise in front of HQ and product events into
  bug reports.
- JSON Lines rather than text, so one reader covers every component and an agent can
  filter without regexes. Third-party logs (cloudflared) stay text and are no longer
  parsed for state.
- No localization of log lines. The event name is the stable part; the chrome around it is
  bilingual.
- Permissions are fixed by migration, not only for new files: the exposure in 1.4 is in
  files that already exist.
- The token is not rotated automatically. After the migration the old copies are readable
  by the owner only, the same as the token file. Rotating it would unpair any device that
  holds the master token, which is the user's call.
