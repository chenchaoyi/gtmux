# diagnostics — status you can read, logs you can trust, and nothing left world-readable

## Why

On 2026-09-19 a phone could not pair with a Mac while the phone already paired with it
kept working. Answering "why" took code reading and fresh probes, and the phone's own
message is still unknown, because none of the three parties kept a record: the phone logs
nothing, the menu bar app logs nothing unless started by hand with a debug variable, and
serve writes nothing at all once it is running. A rejected pairing code leaves no trace
anywhere.

The same investigation found the pairing window deciding whether the tunnel was down by
matching phrases in cloudflared's log, for either backend. For a Direct user that log
belongs to a tunnel they do not run, so the verdict it shows is a function of whatever
that file happens to hold. cloudflared publishes the real number on a metrics endpoint
nobody reads, and the Direct client publishes nothing.

And it found gtmux's data directory readable by every account on the Mac: 88 MB, with the
serve token in 398 places (the log, fixed in v1.0.33), conversation summaries in the
event journal, error lines lifted from agent transcripts, and images sent from the phone.
The token file itself is 0600; the files around it undid that.

These are one problem. gtmux has a well-built event journal for HQ and nothing for its own
diagnostics, so each component improvised: some write text, some write nothing, some parse
others' text for state, and every file takes the default mode. The full audit is in `design.md` section 1.

## What Changes

- Four kinds of record, each with one shape and one reader: **status** (what is true now,
  one small JSON file per component, overwritten), **logs** (what happened, for someone
  debugging, JSON Lines in one schema), the **journal** (`events.jsonl`, unchanged, for
  HQ), and **user data** (unchanged). Surfaces read status and never parse a log.
- A shared logger for the Go components: one schema, four levels, redaction of
  credentials at the sink, rotation by rename (2 MB × 3), one debug switch
  (`GTMUX_DEBUG`) with the old variables kept as aliases.
- serve records its own life: start and stop, rejected tokens (aggregated), pairing codes
  minted, redeemed and rejected, devices revoked, failed sends and pushes, handler panics.
- Tunnel status for both backends: the Direct client reports its connection and an
  end-to-end probe of its own pairing URL; Standard's state comes from cloudflared's
  metrics endpoint. The pairing window and `gtmux doctor` read it. No log text is parsed.
- Every file gtmux owns under its data and config directories becomes 0600 and every
  directory 0700, by a one-time migration of what already exists and by helpers for what
  is written next. The retired `hq-feed/spool.jsonl` is removed.
- A reader, `gtmux logs`, merging every component by time. The menu bar writes to the same
  schema. The phone and iPad keep a small redacted buffer under Settings → Diagnostics.
  `gtmux doctor --bundle` packs logs and status for a bug report. Nothing is uploaded.
- Guards in `check-design.sh`: no new world-readable file modes in `internal/`, and no
  surface reading a `*.log` for state.

Delivered in three phases (design section 9); each ships on its own.

## Surfaces

- 终端 (terminal / attach): writes: serve, the tunnel client, hook, restore and command failures log
  through the shared logger; serve and the tunnel publish status. Reads: `gtmux logs`
  (new command), `gtmux doctor` rows from status, `gtmux doctor --bundle`. Remote attach
  logs its connection failures on the client side (`cli` component).
- 菜单栏 (menubar): the pairing window and the Preferences door status read
  `status/tunnel.json` instead of cloudflared's log; the app writes `logs/menubar.jsonl`
  in the shared schema and mirrors to the unified logging system.
- 手机 (phone): a ring buffer of the last 500 redacted entries (pairing attempts with their
  classified cause and HTTP status, connection changes, push registration), shown and
  shareable under Settings → Diagnostics. Phase 3.
- iPad: the same implementation as the phone, reached from the regular shell's
  settings. Phase 3.
- Web: not applicable for persistent logs. The mirror runs in other people's browsers
  under share links and deliberately keeps nothing there; it logs to the browser console
  only, as today.

## Impact

- New capability spec `diagnostics`; additions to `menu-bar-app` and `env-doctor`.
- New command `gtmux logs` (CLAUDE.md command list, the help table, a `docs/cli.md`
  section in both halves).
- `api/contract.md`: unchanged. Status is read from files on the Mac, not over HTTP.
- File modes under `~/.local/share/gtmux/` and `~/.config/gtmux/` change on first start of
  the new version. Anything outside gtmux that reads those files as another user stops
  being able to, which is the point.
