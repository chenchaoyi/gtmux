# remote-terminal-client Specification

## Purpose
TBD - created by archiving change remote-terminal-client. Update Purpose after archive.
## Requirements
### Requirement: Attach to a remote pane by target, resolving scope

`gtmux attach <target>` SHALL open a remote tmux pane in the local terminal as a raw,
interactive passthrough. The target SHALL be a guest share link
(`https://host/#g=<token>` → GUEST bearer; legacy `#t=` accepted) or a host + `--token <tok>` (→ OWNER bearer).
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

#### Scenario: Guest attaches an allowed pane

- **WHEN** the user runs `gtmux attach https://host/#g=<token> %N` for a view-allowed pane
- **THEN** the attach opens; if the pane is on the input allowlist it is interactive, otherwise it is read-only

#### Scenario: Guest is refused a non-viewable pane

- **WHEN** a guest attaches a pane not on its view allowlist
- **THEN** the server refuses the WebSocket upgrade (no PTY is spawned) and the client exits with a clear "not shared" message

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

`GET /api/attach` SHALL, before spawning any PTY, authorize the caller: an owner may
attach any pane; a guest may attach ONLY a view-allowed pane. Once bridged, the server
SHALL DROP `INPUT`/`RESIZE` frames for a pane the caller may not type into (a view-only
guest pane is read-only) — never trusting the client. The bridge SHALL bound its
buffering and honor client `PAUSE`/`RESUME` flow control (pausing its PTY read on
`PAUSE`) so a flooding pane cannot grow memory without bound.

#### Scenario: A view-only guest cannot type

- **WHEN** a guest attached to a view-only pane sends input
- **THEN** the server drops the input frame — the pane is not written to — even if the client sent it

#### Scenario: A flooding pane does not exhaust memory

- **WHEN** the attached pane floods output faster than the client consumes and the client sends `PAUSE`
- **THEN** the server stops reading the PTY until `RESUME`, bounding buffered memory (no unbounded growth)

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

