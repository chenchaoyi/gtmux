# diagnostics — one local log for what gtmux saw and what it did

## Why

On 2026-09-19 a phone could not pair with a Mac while the phone already paired with it
kept working. Answering "why" took code reading and fresh probes, and the phone's own
message is still unknown, because none of the three parties kept a record: the phone logs
nothing, the menu bar app logs nothing unless started by hand with a debug variable, and
serve writes nothing at all once it is running. A rejected pairing code leaves no trace
anywhere.

The same investigation found the pairing window deciding whether the tunnel was down by
matching phrases in cloudflared's log, for either backend. For a Direct user that log
belongs to a tunnel they do not run. cloudflared publishes the real number on a metrics
endpoint nobody reads, and the Direct client publishes nothing.

It found that apart from seven acts the journal audits for HQ, nothing gtmux does to the
Mac is recorded: a focus, a phone's send, a device paired or revoked, hooks installed, an
update, a change `doctor --fix` made, files disk hygiene deleted.

And it found gtmux's data directory readable by every account on the Mac: 88 MB, with the
serve token in 398 places (the log, fixed in v1.0.33), conversation summaries in the
event journal, error lines lifted from agent transcripts, and images sent from the phone.

These are one problem. gtmux has a well-built event journal for HQ and nothing for its own
diagnostics or its own acts, so each component improvised, and every file took the
default mode. The full audit is in `design.md` section 1.

## What Changes

- One local log store, in the spirit of the macOS unified log: every gtmux process writes
  to `~/.local/share/gtmux/logs/`, one file per day, one schema, English. It holds
  diagnostics and a trail of every action that changes something, with who started it
  (terminal, HQ, another agent, the menu bar, a phone or browser by device, the system),
  what it acted on, and how it ended. No user content; credentials replaced where the line
  is written.
- Growth is bounded by the store itself: 30 days or 100 MB, whichever comes first, cleaned
  by whichever process writes the first entry of a new day and by serve's slow tick, with
  an in-day guard that names a runaway writer. Every other growing store gets one written
  bound and one owner, and every deletion is itself logged.
- `gtmux logs`, the store's reader, as `log show` and `log stream` are the system's: merged
  by time, filtered by time, component, level, actor or event, followed live, or printed
  as JSON for an agent.
- `gtmux doctor` gains a `Logs` section: store size and retention, a runaway writer,
  recent errors by component, file modes, and every other store against its bound, with
  `--fix` running cleanup and narrowing modes.
- Status files for current state, read by surfaces instead of log text. Tunnel status for
  both backends (the Direct client's connection and an end-to-end probe; cloudflared's
  metrics for Standard), read by the pairing window and doctor.
- Two roots, each meaning one thing: `~/.config/gtmux/` for what a person set or would
  carry to a new machine, `~/.local/share/gtmux/` for what gtmux generates, with
  misplaced files moved, the data root given `logs/`, `status/` and `cache/`, retired
  leftovers removed, and every path built in one place in code.
- Private by default: a `077` umask at every gtmux entry point, conventional modes kept
  for files gtmux edits but does not own, and both roots narrowed to 0600 / 0700 on
  serve start and every hygiene sweep. The token is not rotated.
- The phone and iPad keep a small redacted buffer under Settings → Diagnostics.
  `gtmux doctor --bundle` packs the logs and status for a bug report. Nothing is uploaded.

Delivered in three phases (design section 11); each ships on its own.

## Surfaces

- 终端 (terminal / attach): serve, the tunnel client, the hook and every command write to
  the store, diagnostics and actions; serve and the tunnel publish status. Read with
  `gtmux logs` (new) and `gtmux doctor`'s `Logs` and tunnel rows; `gtmux doctor --bundle`.
  Remote attach records its sessions and failures on the client side.
- 菜单栏 (menubar): writes its entries (actions from the menu, failed CLI calls, failed
  code mints, reachability verdicts, notifications posted) into the same store and mirrors
  them to the unified log for Console.app; the pairing window and the Preferences door
  status read `status/tunnel.json` instead of cloudflared's log.
- 手机 (phone): a buffer of the last 500 redacted entries in the same schema (pairing
  attempts with their classified cause and HTTP status, connection changes, push
  registration, sends), under Settings → Diagnostics with copy and share. Its actions
  against the Mac are also in the Mac's store, recorded by serve with the device as actor.
- iPad: the same implementation as the phone, reached from the regular shell's settings.
- Web: not applicable for its own log. The mirror runs in other people's browsers under
  share links and keeps nothing there; its actions against the Mac are recorded by serve
  with the browser or guest link as actor.

## Impact

- New capability spec `diagnostics`; additions to `menu-bar-app` and `env-doctor`.
- New command `gtmux logs` (CLAUDE.md command list, the command table, a `docs/cli.md`
  section in both halves).
- `api/contract.md`: unchanged. Status and logs are read from files on the Mac.
- File modes under `~/.local/share/gtmux/` and `~/.config/gtmux/` change on first start of
  the new version; another account on the Mac can no longer read them.
