# remote-terminal-client Specification

## Purpose
TBD - created by archiving change remote-terminal-client. Update Purpose after archive.
## Requirements
### Requirement: Attach to a remote pane by target, resolving scope

`gtmux attach <target>` SHALL open a remote tmux pane in the local terminal as a raw,
interactive passthrough. The target SHALL be a host + `--token <tok>` (→ OWNER bearer) or
a pair link that enrolls the terminal as an owner device. A guest share link
(`https://host/#g=<token>`, `#code=`, or `--code`) still resolves to a GUEST bearer, and
the server SHALL refuse it (see the scope requirement below).
The client SHALL verify reachability + token, resolve scope from `GET /api/share`
(`all:true` ⇒ owner), and connect a WebSocket to `GET /api/attach?id=%N`. It SHALL stay
cgo-free.

When no `%pane` is given, the client SHALL resolve the pane as follows: if exactly one
pane is attachable it SHALL auto-select it; if none are attachable it SHALL error. If
more than one is attachable, then — WHEN stdin is a TTY — it SHALL present a numbered
menu (one row per pane, showing session · agent · status · task) and attach to the
chosen row without requiring the command be re-run; Enter SHALL select the first row and
`q`/`Esc`/EOF SHALL cancel with no attach. WHEN stdin is NOT a TTY, it SHALL instead
print the pane list and exit non-zero (never blocking on input), so scripts stay
deterministic.

#### Scenario: Owner attaches a pane

- **WHEN** the user runs `gtmux attach <host> --token <device-token> %N`
- **THEN** the local terminal enters raw mode and shows the live pane; keystrokes go to the pane and its output renders byte-for-byte, until the user detaches

#### Scenario: A share link is refused a terminal

- **WHEN** the user runs `gtmux attach https://host/#code=<code> %N`, for any pane, granted or not
- **THEN** the server refuses the WebSocket upgrade (no PTY is spawned) and the client exits with the server's reason: a share link cannot open a terminal; open it in a browser

#### Scenario: Interactive pick among multiple panes on a TTY

- **WHEN** the user runs `gtmux attach <host>` (no `%pane`) from an interactive terminal and more than one pane is attachable
- **THEN** the client prints a numbered menu (session · agent · status · task per row), reads a choice, and attaches to that pane directly; pressing Enter picks the first row and `q` cancels without attaching

#### Scenario: Non-TTY stays scriptable

- **WHEN** `gtmux attach <host>` (no `%pane`) runs with stdin NOT a TTY (a pipe or script) and more than one pane is attachable
- **THEN** the client prints the pane list and exits non-zero without prompting, so automation never blocks on input

### Requirement: Raw local terminal with faithful passthrough

While attached the client SHALL put the local terminal into raw mode and passthrough
bytes both directions: local input → the pane, pane output → the local screen,
byte-for-byte (full TUI apps, colors, cursor). It SHALL trap `SIGWINCH` and send the
new size so the remote pane resizes, and it SHALL restore the terminal (cooked mode) on
every exit path (normal detach, error, or signal). The client SHALL send its local
`$TERM` to the server; the server SHALL honor it for the spawned tmux client ONLY when
the remote has terminfo for it (else a safe `xterm-256color` fallback), and SHALL force
a UTF-8 locale on the spawned process so CJK / wide glyphs render (the serve's launchd
environment has no `TERM`/locale of its own).

#### Scenario: Interactive TUI works

- **WHEN** the attached pane runs a full-screen TUI (e.g. an editor, or the agent's UI)
- **THEN** it renders and responds correctly, because bytes pass through unparsed to the real local terminal

#### Scenario: CJK and wide glyphs render (not placeholder dashes)

- **WHEN** the attached pane contains CJK or other wide/multibyte glyphs
- **THEN** they render as the real characters, because the server forces a UTF-8 locale (`LC_CTYPE`) and passes `-u` to tmux — never the `-` placeholders a locale-less environment produces

#### Scenario: Terminal is restored on exit

- **WHEN** the client exits for any reason (detach key, error, Ctrl-C, killed)
- **THEN** the local terminal is returned to its normal (cooked) mode, never left raw

### Requirement: Server-authoritative scope + flow control on the WS bridge

`GET /api/attach` SHALL, before spawning any PTY, authorize the caller: the owner and
paired devices may attach any pane; a guest (share-link) caller SHALL be refused,
whatever panes its link grants. The bridge attaches a tmux client to the pane's whole
session: it draws every pane of the session's current window, and a client that may
type can drive tmux itself (prefix keys, the command prompt) into any session. A
pane-scoped link must reach neither (2026-10-06: both reproduced on an isolated serve).
Until the bridge can carry one pane alone, a guest's surfaces are the browser and phone
views, which are scoped per pane. The bridge SHALL bound its
buffering and honor client `PAUSE`/`RESUME` flow control (pausing its PTY read on
`PAUSE`) so a flooding pane cannot grow memory without bound. Once the server has read a
`PAUSE`, it SHALL start no new PTY read and no new `OUTPUT` frame until `RESUME`: a frame
already being written completes, and bytes a read had already returned (at most one read
buffer) are held and sent after `RESUME`, in order. The pause is per connection, and
repeated `PAUSE` or `RESUME` frames are idempotent. A pause SHALL NOT hold the input
direction or the reading of further frames, and SHALL NOT keep a session open that is
revoked or whose client leaves. Whether a frame may start SHALL be decided at the moment it
would start (under the write lock), so a `PAUSE` read while output waited for that lock
holds the frame. The end of the program SHALL end the session, paused or not, and the program
SHALL be reaped at once: what the server holds, and what is left in the terminal, is sent and
the session ends, without waiting for `RESUME` and without dropping that output. On macOS, a
program may remain in the exiting state while its final PTY output is unread; such a program
waits as any program with held output does, until `RESUME`, or until its client leaves or is
revoked.

#### Scenario: A share link with input granted is still refused

- **WHEN** a guest whose link may view and type into pane %N requests `/api/attach?id=%N`
- **THEN** the server answers 403 before any upgrade, so no tmux client is ever spawned for it

#### Scenario: Revoking the caller ends its open session

- **WHEN** an attached caller's device or share link is revoked while the session is open
- **THEN** within a few seconds the server ends the session, as it refuses any new request with that token, after trying for a bounded time to write an "access revoked" line; a session on the serve's own token is not affected

#### Scenario: Revoking a caller whose client has stopped reading

- **WHEN** a caller is revoked while the server's output to it is blocked because its client is not reading
- **THEN** the session still ends within a few seconds; the "access revoked" line is best effort, waited on for a bounded time only (at most about a second: for the write lock, then for the write), so that client may never receive it

#### Scenario: A flooding pane does not exhaust memory

- **WHEN** the attached pane floods output faster than the client consumes and the client sends `PAUSE`
- **THEN** the server stops reading the PTY until `RESUME`, bounding buffered memory (no unbounded growth)

#### Scenario: Output resumes whole after a pause

- **WHEN** a client pauses, the program then writes a long run of output, and the client resumes
- **THEN** nothing arrives while paused, and after `RESUME` the whole run arrives in order, with nothing lost

#### Scenario: The program ends while paused

- **WHEN** the program in a paused session prints its last output and exits, and no `RESUME` follows
- **THEN** the program is reaped (no defunct process), its last output is delivered, and the
  session ends

#### Scenario: A PAUSE that arrives while output waits for the write lock

- **WHEN** output has been read and waits for the write lock (a cursor frame holds it), and a
  `PAUSE` is read before the lock comes free
- **THEN** that output is not sent until `RESUME`, and none of it is lost

#### Scenario: A paused session still ends

- **WHEN** a paused caller is revoked, or its client disconnects
- **THEN** the session ends as it would unpaused; input sent while paused reaches the program at once

### Requirement: Attach pairs a terminal as an owner surface

`gtmux attach` SHALL support the owner-pairing medium: an attach target whose
fragment carries an enroll code (`#c=<code>`) SHALL be redeemed once via
`POST /api/enroll` (device name = the local hostname) for an OWNER device token,
persisted locally (`~/.config/gtmux/remotes.json`, mode 0600, keyed by host), and
the attach proceeds with `full` scope. A later bare `gtmux attach <host>` SHALL
reuse the persisted token for that host before requiring `--token`. Guest share
links (`#g=`, legacy `#t=`) SHALL keep their existing behavior. Revoking the device on the host
(`gtmux pair revoke`) SHALL invalidate the persisted token immediately (the next
request fails auth).

#### Scenario: Pair a terminal with the one-liner

- **WHEN** the user runs the printed `gtmux attach 'https://host/#c=<code>'` on
  another computer
- **THEN** the code is redeemed for an owner device token, stored in
  remotes.json (0600), and the session attaches with full scope

#### Scenario: Subsequent attach needs no credential

- **WHEN** the same user later runs `gtmux attach host` with no flags
- **THEN** the persisted token authenticates and the attach proceeds as owner

#### Scenario: Host-side revocation cuts the terminal off

- **WHEN** the owner revokes that terminal's device on the host
- **THEN** the persisted token stops authenticating (`401`) on its next use

### Requirement: Server sends the tmux cursor over the attach bridge

The attach WebSocket bridge SHALL support a server→client `OpCursor` frame carrying the
bridged pane's tmux cursor position and alt-screen flag (`{x, y, alt}`), sampled from tmux
(`#{cursor_x}`, `#{cursor_y}`, `#{alternate_on}`) on a small cadence and after output
batches. The frame SHALL be additive and non-blocking: it never stalls the PTY output
pump, an old client ignores the unknown opcode, and a client that receives no `OpCursor`
frames behaves exactly as before.

#### Scenario: Cursor frames accompany the stream

- **WHEN** an owner attaches a pane and types at a shell prompt
- **THEN** the server sends `OpCursor` frames reflecting the tmux cursor as it moves, without interrupting or delaying the `OpOutput` byte stream

### Requirement: Predictive local echo (opt-in, adaptive, honest)

`gtmux attach` SHALL offer opt-in predictive local echo (a `--predict` flag / config, OFF
by default) that shows the user's own printable keystrokes and backspaces IMMEDIATELY in a
distinct **unconfirmed** style (underlined/dim), before the server echoes them, so typing
does not wait for the round-trip. The actual keystroke SHALL still be sent to the pane
unchanged, and the server output SHALL always be authoritative: outstanding predictions
SHALL be erased before authoritative output is applied, so a mispredicted character is
never left as if real.

Prediction SHALL be gated for honesty: only printable characters and backspace are
predicted; prediction is adaptive (only when the measured round-trip exceeds a threshold —
a fast/LAN link shows none); it SHALL NOT predict when the pane is in the alternate screen
(`alt=true`, a full-screen TUI); and any state-changing key (Enter, ESC, arrows, Ctrl-C,
Tab) SHALL end the prediction epoch — clearing outstanding predictions and pausing until
the next authoritative cursor. On any uncertainty the client SHALL fall back to plain
passthrough.

#### Scenario: Typing at a prompt over a slow link with --predict

- **WHEN** predict is enabled, the round-trip is high, and the user types a printable character at a cooked prompt
- **THEN** the character appears immediately in the unconfirmed (underlined) style and is replaced by the authoritative echo when the server output arrives

#### Scenario: A mispredicted character is erased, not trusted

- **WHEN** an outstanding prediction does not match the authoritative output that arrives
- **THEN** the prediction is erased before the real bytes are written — the screen never shows a wrong character as confirmed

#### Scenario: No prediction in a full-screen TUI or on a fast link

- **WHEN** the pane is in the alternate screen (`alt=true`), or the measured round-trip is below the threshold
- **THEN** no prediction is drawn and attach behaves as plain raw passthrough

### Requirement: A slow host is not reported as an unreachable one

The client SHALL distinguish a setup request that TIMED OUT from one that could not
connect, and SHALL say which. The reachability probe has already succeeded by the time the
session list is fetched, so a deadline there means the host is busy, not gone — and the
remedy differs: waiting or naming a pane directly, rather than diagnosing connectivity. The
budget for these setup requests SHALL be generous enough to absorb an ordinarily loaded
host, since the cost of listing sessions scales with what the host is doing and the request
may cross a tunnel; failing early costs the user the session, while waiting longer costs
them nothing they would notice.

#### Scenario: The host is reachable but slow to list sessions

- **WHEN** the session list request exceeds its deadline after the host answered the
  reachability probe
- **THEN** the client says the host is reachable but busy, and offers naming the pane
  directly instead of listing

#### Scenario: The host cannot be reached

- **WHEN** the connection itself fails
- **THEN** the client reports that, not a timeout
