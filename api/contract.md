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

`agent` object (stable fields; see `internal/radar/agents.go` `agentJSON`):

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

### `GET /api/digest` — the fleet's cognitive digest (read-only, OWNER only)

Byte-identical to `gtmux digest --json`: one row per agent carrying what a supervisor
needs to triage — `goal` / `last` / `ask` on top of the radar's state. This is the
`agent-digest` capability's wire form; the phone's HQ page reads it for the
"who is blocked, and on what" decision cards.

```
200 [{"pane_id":"%17","loc":"api:0.0","agent":"Claude Code","source":"tmux","status":"waiting","kind":"permission","goal":"refactor auth","last":"split verifyToken()","ask":"run the test suite?","since":1784720000,"tok":5100,"ctx":0.62}, …]
403 {"error":"forbidden: not shared"}   // guest scope
503 {"error":"digest unavailable"}      // DigestJSON dep not wired
```

### `GET /api/usage` — token accounting + subscription windows (read-only, OWNER only)

Byte-identical to `gtmux usage --json` (the `usage-watch` capability): per-session token
totals and rates, the plan's real limit windows with `pct_used`, and the machine
resource snapshot (`resource-watch`). Owner-only — it exposes the whole fleet's budget.

```
200 {"sessions":[…],"limits":{"windows":[{"label":"week (all models)","pct_used":41,"reset_at":"…"}]},"resource":{"machine":{"warn":"","disk_free_gb":210,"mem_tier":"ok"}}}
403 {"error":"forbidden: not shared"}
503 {"error":"usage unavailable"}
```

### `GET /api/hq/board` — the supervisor's situation board (read-only, OWNER only)

The synthesis gtmux HQ maintains by hand (`~/.config/gtmux/hq/notes/board.md`) so its
picture of the fleet survives a context reset — the one considered assessment anywhere in
the product, and what the mobile HQ page shows instead of re-listing the radar
(`hq-command-page`). Read-only: the board is HQ's own working memory, not something a
client edits.

A supervisor that has never written a board is ORDINARY, not an error — the response is
`200` with `exists:false`, and a client degrades to its deterministic assessment line.
Text is capped (128 KB, head kept — a board leads with its freshness line and current
focus).

```
200 {"exists":true,"updated_at":1784720000,"text":"# gtmux HQ — situation board\n…"}
200 {"exists":false}              // no board written, or no HQ home on this Mac
403 {"error":"forbidden: not shared"}   // guest scope
```

### `GET /api/hq/events` — the fleet event ledger (read-only, OWNER only)

The severity-tagged lifecycle ledger (`internal/events`), **newest first** — history,
where `/api/agents` only has the present instant. Same records `gtmux events --json`
prints, in feed order rather than log order.

| param | default | meaning |
|---|---|---|
| `severity` | *(all)* | floor: `routine` \| `notable` \| `important` — that tier **and above** |
| `limit` | `40` | max records, clamped to `200`; a junk value falls back to the default rather than erroring |

Always a JSON array (never `null`); an unreachable/absent ledger reads as `[]`, since
"nothing happened" and "can't tell you" are the same thing to a reader.

```
200 [{"ts":1784720000,"seq":108,"event":"Waiting","state":"waiting","pane":"%17","loc":"api:0.0","session":"api","agent":"Claude Code","kind":"permission","summary":"run the test suite?","severity":"important"}, …]
403 {"error":"forbidden: not shared"}   // guest scope
```

Both HQ surfaces are refused to a guest for the same reason `/api/digest` and
`/api/usage` are: they carry the WHOLE fleet plus HQ's private assessment, which are
owner surfaces and never part of a shared scope.

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

### `GET /api/attach?id=%N` — attach a pane's PTY (WebSocket, WRITE)

Upgrades to a **WebSocket** that bridges a tmux pane's PTY to the caller — the
`gtmux attach` client puts the local terminal in raw mode and passes bytes through
both ways. **Authed + scope-gated**: an owner may attach any pane; a `guest` token
may attach ONLY a view-allowed pane (else the upgrade is **refused 403**), and
`INPUT`/`RESIZE` frames are **dropped server-side** for a pane it may not type into
(a view-only pane is read-only). Scope enforcement is server-side; a client flag
never widens it.

Wire format: **binary** frames, first byte an opcode, payload from index 1 (no
base64 — raw PTY bytes):

| dir | opcode | payload |
|---|---|---|
| client→server | `i` INPUT | raw key bytes → the pane |
| client→server | `r` RESIZE | `{"cols":C,"rows":R}` → `pty.Setsize` |
| client→server | `p` PAUSE / `R` RESUME | flow control (reserved; MVP relies on natural WS backpressure) |
| server→client | `o` OUTPUT | raw PTY bytes → the local screen |

The server spawns `tmux -u attach-session` for the pane inside a `creack/pty` PTY and
streams the master byte-for-byte. An optional **`&term=<name>`** query carries the
client's local `$TERM`; the server honors it for the spawned tmux client only when the
remote has terminfo for it (validated via `infocmp`, name sanitized) and otherwise falls
back to `xterm-256color`. It also forces a UTF-8 locale (`LC_CTYPE`) on the spawned
process so CJK / wide glyphs render instead of placeholder dashes (the serve's launchd
env has no `TERM`/locale). See `docs/design/remote-attach-research.md`.

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

The server BINDS the token to the caller's enrolled device — it stamps `deviceId`
from the bearer token's roster entry (never the request body), so revoking that
device drops the token (see `/api/devices/revoke`). A token registered without a
roster entry (e.g. the master token, or one persisted before this binding existed)
has an empty `deviceId` and is treated as **unlinked** (legacy).

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

### `POST /api/push/unregister` — stop pushing to a device

Drops a device's tokens from this Mac, so it stops pushing to a phone that has
**removed this server**: the APNs `token` stops alerts + silent-badge pushes, and
the optional `activityToken` stops Live Activity lock-screen updates (the Mac also
pushes an `end` so a card it was keeping alive disappears). The app calls it
best-effort when you delete a paired Mac. Each Mac keeps its own token set, so
removing one paired server never affects push from the others.

```
body: {"token":"<device-token>","activityToken":"<live-activity-token>"}
200 {"status":"ok"}                  // idempotent: 200 even if never registered
400 {"error":"invalid token"}        // both token and activityToken empty / bad body
503 {"error":"push not configured"}  // server started without push support
```

At least one of `token` / `activityToken` must be present; either may be omitted.

### `POST /api/push/test` — send a test notification

Sends a test push to **every** registered device (so the settings screen can
verify notifications end-to-end). No body.

```
200 {"sent":N}                       // N = devices the relay accepted
405 {"error":"method not allowed"}   // non-POST
503 {"error":"push not configured"}  // server started without push support
```

### `GET /api/push/tokens` — list the registered push tokens (MASTER only)

Returns the stored push tokens REDACTED (a short `tokenPrefix`, never the full
secret) with their device binding, so the Mac's own CLI (`gtmux devices --push`) can
inspect + clean up the store. **Master-only** — a device/guest is `403`.

```
200 {"tokens":[{"deviceId":"<id|empty>","tokenPrefix":"abc123…","platform":"ios","env":"sandbox","kinds":["waiting"]}]}
403 {"error":"forbidden: host-only"} // a device/guest caller
503 {"error":"push not configured"}
```

An empty `deviceId` marks an **unlinked** (legacy) token.

### `POST /api/push/forget` — drop push tokens (MASTER only)

Clears tokens by selector — `deviceId` (that device's tokens), `orphans` (only
unlinked legacy tokens), or `all` (every token) — and persists. Backs
`gtmux devices --forget-push <id|orphans|all>`. **Master-only** — a device/guest is
`403`.

```
body: {"deviceId":"<id>"} | {"orphans":true} | {"all":true}
200 {"forgotten":N}
400 {"error":"give deviceId, orphans, or all"}
403 {"error":"forbidden: host-only"}
503 {"error":"push not configured"}
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
200 {"devices":[{"id":"…","name":"…","enrolledAt":<epoch>,"lastSeen":<epoch>?,"scope":"…"?}, …]}
503 {"error":"enrollment not configured"}
```

`scope` is `"guest"` for a share link, absent/empty for an owner device (guest entries
also carry `viewPanes`/`inputPanes`/`expiresAt`, below).

### `POST /api/devices/revoke` — revoke a device's token now

Also unregisters any push token bound to that device (`deviceId`), so a revoked
device stops receiving notifications immediately — no separate `/api/push/forget`.

```
body: {"id":"<deviceId>"}
200 {"revoked":true|false}                       // false = no such device
400 {"error":"invalid request"}
405 {"error":"method not allowed"}
503 {"error":"enrollment not configured"}
```

## Sharing (guest links — the SHARE track of pair-share-model)

Every credential carries a SCOPE: the master token and enrolled devices are FULL
(the owner's own surfaces — the PAIR track); a share link is a GUEST. Each guest
link carries its OWN per-pane scope — a **view** allowlist and an **input**
allowlist (input ⊆ view) — plus an optional expiry; an expired link fails auth
(`401`) exactly like a revoked one. Typing additionally requires the host-level
consent switch. All gates are server-side and authoritative.

### `GET /api/share` — the caller's own capability (any scope)

```
200 {"input":true,"all":true,"panes":[],"view_panes":[]}          // full caller
200 {"input":false,"panes":[],"view_panes":["%1","%2"]}           // guest, view-only
200 {"input":true,"panes":["%1"],"view_panes":["%1","%2"]}        // guest, may type %1
```

A guest's reply resolves from ITS OWN link scope. `input` is true only when the
caller can actually type somewhere (consent on AND a non-empty input list). UIs
mirror (never widen) this.

### `GET/POST /api/share/config` — the host policy (master only)

```
GET  200 {"enabled":false,"panes":[],"view_panes":[]}
POST body: {"enabled":bool?,"panes":[…]?,"view_panes":[…]?}   // partial update
     200 <the updated state>
403 {"error":"forbidden: host-only"}                          // device/guest caller
```

`enabled` is the consent master switch for ALL guest typing. The pane lists are
the legacy GLOBAL lists, kept with two roles (pair-share-model): the TEMPLATE
copied into links minted without explicit scope, and a BROADCAST — a POST that
changes them also replaces every existing link's per-link lists (the pre-per-link
behavior, preserved for older UIs).

### `POST /api/share/new` — mint a guest link (master only)

```
body: {"label":"Alice","view":["%1","%2"]?,"input":["%1"]?,"expiresInSec":86400?}
200 {"token":"<guest-token>","id":"<id>","name":"Alice"}
```

Omitted `view`/`input` copy the current template; omitted `expiresInSec` = never
expires. Input is normalized into view (input ⊆ view).

### Authorization (owner-remote-admin)

The SHARE-management endpoints (`/api/share/config`, `/api/share/new`,
`/api/share/set`, `/api/share/link`) and `GET /api/devices` are **FULL-only**: the
master token OR an owner device (a paired phone/browser/terminal). A `guest` is
refused (`403`) — so an owner can manage sharing remotely, and a guest cannot list
the roster (this closes a prior unguarded path). `POST /api/devices/revoke` is
SCOPED: a master may revoke any entry; an owner device may revoke ONLY a guest link
(`403 forbidden: paired devices are managed on the Mac` for a paired device); a
guest is refused. Toggling the remote-access door stays a local Mac operation.

### `POST /api/share/set` — edit ONE link's scope (full: master or owner device)

```
body: {"id":"<id>","view":[…]?,"input":[…]?,"expiresInSec":N?,"clearExpiry":true?}
200 {"id":"…","name":"…","viewPanes":[…],"inputPanes":[…],"expiresAt":<epoch|0>}
404 {"error":"unknown share link"}
```

Per-facet replace: an omitted field leaves that facet untouched. Only guest
entries are editable.

`GET /api/devices` additionally carries each guest entry's `viewPanes`,
`inputPanes`, and `expiresAt` (additive; absent on owner devices).

### `GET /api/share/link?id=<id>` — re-copy a link's URL (full only)

```
200 {"id":"…","label":"…","token":"<guest-token>"}   // build <base>/#g=<token>
403 {"error":"forbidden: not shared"}                // a guest caller
404 {"error":"unknown share link"}                   // no such guest link
```

Re-hands a guest link's token so an owner can re-copy the share URL after minting
(a link is no longer view-once). Only guest links resolve; a paired device's token
is never returned.
