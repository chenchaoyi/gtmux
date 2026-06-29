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
- Mostly read-only — except **`POST /api/send`**, which **writes** to a terminal
  (`tmux send-keys`). It is gated only by the bearer token, so a leaked token
  allows running commands on the Mac. `/api/focus` only *selects* a pane.

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

`tmux capture-pane -e -p -S -2000` of the pane (the visible screen plus up to
2000 lines of scrollback), with `-e` keeping ANSI SGR so the mirror renders
colors. Trailing blank rows are **preserved** (the capture is not right-trimmed)
so the cursor offset below anchors to the true bottom. `id` is URL-encoded
(`%` → `%25`, so pane `%12` is `?id=%2512`).

```
200 {"id":"%12","text":"…current screen…","cursor":{"x":4,"up":0,"visible":true}}
400 {"error":"missing id"}
404 {"error":"pane not found"}     // pane no longer exists
```

`cursor` is **optional** (omitted when the server can't resolve a cursor for the
pane). It is **bottom-anchored** so a client can place a cursor block without
knowing the capture's top:

| field | type | meaning |
| --- | --- | --- |
| `x` | int | cursor column, 0-based from the left |
| `up` | int | rows **up from the last captured line** (`pane_height-1-cursor_y`); `0` = the bottom row |
| `visible` | bool | whether the pane's cursor is currently shown |

### `POST /api/focus?id=%N` — select a pane locally

Selects that window+pane in tmux and brings its terminal tab forward on the Mac
(the same jump the watch TUI does on Enter). Injects no input. Body is ignored.

```
200 {"status":"ok"}
400 {"error":"missing id"}
404 {"error":"focus failed"}       // not a pane id, or pane is gone
405 {"error":"method not allowed"} // non-POST
```

### `POST /api/send` — type into a pane (WRITE)

Types into a pane via `tmux send-keys`. JSON body — supply **exactly one** of:

| field | type | meaning |
| --- | --- | --- |
| `id` | string | the target pane id (`%N`), required |
| `key` | string | a NAMED control key, allow-listed: `Enter`, `C-c`, `Escape`, `Tab`, `Up`, `Down`, `Left`, `Right`, `BSpace`, `C-d`, `C-z`, `C-l` |
| `text` | string | literal text typed with `send-keys -l` (never interpreted as keys) |
| `enter` | bool | with `text`: also press Enter afterward |

```
200 {"status":"ok"}
400 {"error":"missing id" | "nothing to send" | "send failed: …"}  // gone pane / key not allowed
405 {"error":"method not allowed"}                                 // non-POST
503 {"error":"input not available"}                                // Send not wired
```

### `POST /api/upload` — attach a file (WRITE)

`multipart/form-data` with a `file` part (≤ 30 MB). Saves it on the Mac under
`~/.local/share/gtmux/uploads/<rand>-<name>` and returns its path, so the phone
can hand a photo/file to an agent by path (e.g. send `look at <path>`).

```
200 {"path":"/Users/…/.local/share/gtmux/uploads/ab12cd34-photo.jpg"}
400 {"error":"missing file" | "bad upload (too large?)"}
405 {"error":"method not allowed"}
503 {"error":"upload not available"}
```

### `GET /api/icon?agent=<name>` — agent identity icon (PNG)

Returns a PNG of the named agent's identity icon, sourced from the user's
**installed app** on the Mac (the same icon the menu-bar app shows — nothing
third-party is bundled). The mobile app uses it for agent avatars.

```
200 image/png   (Cache-Control: public, max-age=86400)
404 {"error":"no icon"}            // unknown agent / no resolvable icon
503 {"error":"icons not available"}  // Icon dep not wired
```

### `GET /api/diff?id=<pane>` — what the agent changed (read-only)

Returns a unified `git diff` (working tree vs `HEAD`, plus a list of untracked
files) of the pane's current working directory — for reviewing an agent's edits
from the phone. `diff` is `""` when the cwd isn't a git repo. Capped at ~400 KB.

```
200 {"id":"%12","diff":"# branch main\ndiff --git a/… …"}
400 {"error":"missing id"}
404 {"error":"diff failed: pane not found"}
503 {"error":"diff not available"}   // Diff dep not wired
```

### `GET /api/transcript?id=%N` — the pane's parsed chat history (read-only)

Parses the agent's on-disk session log for the pane's cwd/session into an ordered
list of turns (one user prompt → the agent's reply, with its tool calls folded
into collapsible steps). Drives the phone's and browser's "对话/chat" view. `[]`
when the pane has no resumable session or no agent log. See the `chat-transcript`
capability for the parsing rules (multi-segment replies, reject-feedback, harness
stripping, incremental cache).

```
200 [ {turn}, … ]    // application/json
400 {"error":"missing id"}
404 {"error":"transcript failed: <reason>"}
503 {"error":"transcript not available"}   // Transcript dep not wired
```

`turn` object:

| field | type | meaning |
| --- | --- | --- |
| `prompt` | string | the user's instruction that opened the turn |
| `response` | string | the full reply — all segment texts joined by a blank line (back-compat / simple consumers) |
| `segments` | array? | the reply in chronological order; each item is one assistant text bubble plus the tool steps that ran AFTER it (text → tools → text → …) |
| `time` | string? | the prompt's wall-clock timestamp (RFC3339, as logged by the agent); omitted when the log carried none |

`segment` = `{"text":string?, "steps":[{step}]?}`; `step` =
`{"kind":"tool", "title":"Edit|Bash|exec_command|…", "detail":"<short arg summary>"?}`.

### `GET /api/options?id=%N` — a waiting pane's interactive choices (read-only)

Parses a pane that is `waiting` on a numbered prompt (the SAME parser the
menu-bar/CLI use) into its `1/2/3` options, for the approval card. `options` is
`[]` when nothing parses.

```
200 {"options":[{"n":1,"label":"Yes"},{"n":2,"label":"Yes, and don't ask again"},{"n":3,"label":"No"}]}
400 {"error":"missing id"}
404 {"error":"pane not found"}
```

### `GET /api/theme` — the host terminal's resolved appearance (read-only)

Returns the Mac terminal's resolved colors + font so the mirror can match the
user's real terminal (see the `terminal-theme` capability). `source` is
`ghostty | iterm2 | default`.

```
200 {"source":"ghostty","background":"#17171a","foreground":"#d4d2cc","cursor":"#d4d2cc","palette":["#…", … 16],"fontFamily":"Hack","fontSize":13}
503 {"error":"theme not available"}   // Theme dep not wired
```

### `GET /api/events` — live updates (Server-Sent Events)

`text/event-stream`. Lets the app react to changes instead of polling. On
connect the server sends one `agents` event to sync; thereafter:

| event | data | when |
|---|---|---|
| `agents` | `{"rev":N}` | the agent set/status changed — **refetch `/api/agents`**. `rev` is monotonic. |
| `alert` | `{"pane","kind","agent","loc","task","repeat"?}` | a transition: `kind:"waiting"` (any→waiting, needs you) or `kind:"done"` (working→idle). Also the push trigger. `repeat:true` marks a **re-nudge** — the pane has stayed `waiting` past the re-nudge interval (~5 min) without you acting, so it re-alerts/re-pushes ("still needs you") until you respond. |
| `ping` | `{}` | ~20s heartbeat to keep the stream alive. |

The server re-snapshots agents every ~1500ms (in step with the watch TUI).
SSE only signals *that* something changed; `/api/agents` stays the one
authoritative payload (no second data shape on the wire).

### `POST /api/push/register` — register a device for push

Stores a device push token so the server can forward `alert`s (waiting/done) as
lock-screen notifications even when the app is closed. Tokens persist on the Mac
(`~/.config/gtmux/push-tokens.json`, `0600`); the relay stays stateless.

```
body: {"token":"<device-token>","platform":"ios","kinds":["waiting","done"]}
200 {"status":"ok"}
400 {"error":"invalid token"}        // missing token / bad body
503 {"error":"push not configured"}  // server started without push support
```

`kinds` filters which alert kinds this device wants (`waiting` = needs-you,
`done` = finished). Omit or send `[]` for **all** kinds. Lets the phone opt out
of e.g. completion notifications from the settings screen.

Delivery path: `gtmux serve` → **push relay** (`--relay-url`, holds the APNs
key) → APNs → device. The relay's own contract is in `relay/README.md`. APNs is
delivered by Apple over any network, so push arrives even when the phone is off
the VPN; only the live view/control needs the tunnel.

**Quick-reply actions.** A `kind:"waiting"` push is tagged with the APNs
`category:"AGENT_WAITING"`, so iOS shows action buttons (`1 Yes` / `2 Always` /
`3 No`) on the notification. Tapping one POSTs the answer to `/api/send`
(`{id:<pane>, text:"1|2|3", enter:true}`) from the background — you unstick a
waiting agent without opening the app. (Requires the relay built with the
category + a device build that registers the category.)

### `POST /api/push/test` — send a test notification

Sends a test push to **every** registered device (so the settings screen can
verify notifications end-to-end). No body.

```
200 {"sent":N}                       // N = devices the relay accepted
405 {"error":"method not allowed"}   // non-POST
503 {"error":"push not configured"}  // server started without push support
```

### `POST /api/push/activity` — register a Live Activity push token

Hands the Mac a Live Activity push token so the relay can push-to-update the
lock-screen tally even when the app is closed (see `push-notifications`).

```
body: {"token":"<activity-push-token>"}
200 {"status":"ok"}
400 {"error":"invalid token"}        // missing token / bad body
503 {"error":"push not configured"}
```

## Enrollment (per-device tokens)

So a phone/browser never carries the master token, a trusted surface mints a
short-lived single-use **enroll code** that the new device redeems for its own
per-device token (see `browser-mirror` pairing). `POST /api/enroll` is the only
**unauthenticated** `/api/*` route besides `/api/health` — the code itself is the
credential. The rest are master-or-device authenticated.

### `POST /api/enroll` — redeem an enroll code (unauthenticated)

```
body: {"enrollCode":"<code>","name":"<device label>"}
200 {"token":"<per-device-token>","deviceId":"<id>"}
400 {"error":"invalid request"}                 // missing/garbled body
401 {"error":"invalid or expired enroll code"}
405 {"error":"method not allowed"}              // non-POST
503 {"error":"enrollment not configured"}
```

### `POST /api/enroll/mint` — mint a fresh enroll code

```
200 {"enrollCode":"<code>","expiresInSec":N}
405 {"error":"method not allowed"}
503 {"error":"enrollment not configured"}
```

### `GET /api/devices` — list enrolled devices (no tokens)

```
200 {"devices":[{"id":"…","name":"…","enrolledAt":<epoch>,"lastSeen":<epoch>?}, …]}
503 {"error":"enrollment not configured"}
```

### `POST /api/devices/revoke` — revoke a device's token now

```
body: {"id":"<deviceId>"}
200 {"revoked":true|false}                       // false = no such device
400 {"error":"invalid request"}
405 {"error":"method not allowed"}
503 {"error":"enrollment not configured"}
```
