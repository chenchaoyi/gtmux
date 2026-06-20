# gtmux remote HTTP contract — `v0`

The single source of truth for the boundary between `gtmux serve`
(`internal/server`) and the remote mobile app (`mobileapp/`). It is
**versioned**: breaking changes bump the version and the prefix. `v0` is
pre-1.0 and may change; once the mobile app ships, changes here are contracts.

## Conventions

- Base: `http://<mac-host>:<port>` reached over a VPN/tunnel (company VPN,
  Tailscale, …). The server binds an intranet/VPN interface, never the public
  internet.
- **Auth:** every `/api/*` route except `/api/health` requires
  `Authorization: Bearer <token>`. The token is generated and persisted
  (`0600`) at `~/.config/gtmux/serve-token` on first `gtmux serve`, or supplied
  with `--token`. Compared in constant time; a bad/absent token → `401`.
- All JSON responses are UTF-8. Errors use `{"error":"<message>"}` with a
  matching HTTP status.
- This is a **read-only** surface (MVP): nothing here writes to a terminal or
  runs a command. `/api/focus` only *selects* an existing pane locally.

## Endpoints

### `GET /api/health` — liveness / reachability (unauthenticated)

For the app's "can I reach this Mac?" self-check. No token required.

```
200 {"service":"gtmux","status":"ok"}
```

### `GET /api/agents` — the agent radar

Returns the **byte-identical** `gtmux agents --json` array, so CLI, menu-bar
app, and mobile app share one shape. Empty array when no tmux server is running.

```
200 [ {agent}, … ]    // application/json
401 {"error":"unauthorized"}
```

`agent` object (stable fields; see `internal/app/agents.go` `agentJSON`):

| field | type | meaning |
|---|---|---|
| `pane_id` | string | tmux pane id (`%N`) — the jump key for `/api/focus` |
| `session` `window` `pane` `loc` | string | tmux location (`loc` = `session:window.pane`) |
| `agent` | string | display name (e.g. `Claude Code`, `Codex`) |
| `status` | string | `working` \| `waiting` \| `idle` \| `running` |
| `task` | string | current task/title, status glyph stripped |
| `latest` | bool | the most-recently-finished pane |
| `activity` | bool | window activity flag |
| `source` | string | `tmux` (native terminals reserved for later) |
| `icon` | string? | identity-icon hint (`.app`/image path); omitted if none |
| `activity_at` `since` | int? | epoch seconds (last activity / current-state start) |

### `GET /api/pane?id=%N` — read a pane's screen (read-only)

`tmux capture-pane -p` of the pane's current screen. `id` is URL-encoded
(`%` → `%25`, so pane `%12` is `?id=%2512`).

```
200 {"id":"%12","text":"…current screen…"}
400 {"error":"missing id"}
404 {"error":"pane not found"}     // pane no longer exists
```

### `POST /api/focus?id=%N` — select a pane locally

Selects that window+pane in tmux and brings its terminal tab forward on the Mac
(the same jump the watch TUI does on Enter). Injects no input. Body is ignored.

```
200 {"status":"ok"}
400 {"error":"missing id"}
404 {"error":"focus failed"}       // not a pane id, or pane is gone
405 {"error":"method not allowed"} // non-POST
```

### `GET /api/events` — live updates (Server-Sent Events)

`text/event-stream`. Lets the app react to changes instead of polling. On
connect the server sends one `agents` event to sync; thereafter:

| event | data | when |
|---|---|---|
| `agents` | `{"rev":N}` | the agent set/status changed — **refetch `/api/agents`**. `rev` is monotonic. |
| `alert` | `{"pane","kind","agent","loc","task"}` | a transition: `kind:"waiting"` (any→waiting, needs you) or `kind:"done"` (working→idle). Also the push trigger. |
| `ping` | `{}` | ~20s heartbeat to keep the stream alive. |

The server re-snapshots agents every ~1500ms (in step with the watch TUI).
SSE only signals *that* something changed; `/api/agents` stays the one
authoritative payload (no second data shape on the wire).

## Reserved (later increments — not yet served)

- `POST /api/push/register` — register a device push token (APNs/FCM/HMS). The
  server forwards `alert`s to a push relay; see `docs/design/remote-mobile.md`.
- `POST /api/send` — Phase 2 control (`tmux send-keys`); gated behind an
  explicit write permission. Out of scope for the read-only MVP.
