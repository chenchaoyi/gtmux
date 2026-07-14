# remote-terminal-client Specification

## Purpose
TBD - created by archiving change remote-terminal-client. Update Purpose after archive.
## Requirements
### Requirement: Attach to a remote pane by target, resolving scope

`gtmux attach <target>` SHALL open a remote tmux pane in the local terminal as a raw,
interactive passthrough. The target SHALL be a guest share link
(`https://host/#t=<token>` → GUEST bearer) or a host + `--token <tok>` (→ OWNER bearer).
The client SHALL verify reachability + token, resolve scope from `GET /api/share`
(`all:true` ⇒ owner), and connect a WebSocket to `GET /api/attach?id=%N`. It SHALL stay
cgo-free.

#### Scenario: Owner attaches a pane

- **WHEN** the user runs `gtmux attach <host> --token <device-token> %N`
- **THEN** the local terminal enters raw mode and shows the live pane; keystrokes go to the pane and its output renders byte-for-byte, until the user detaches

#### Scenario: Guest attaches an allowed pane

- **WHEN** the user runs `gtmux attach https://host/#t=<token> %N` for a view-allowed pane
- **THEN** the attach opens; if the pane is on the input allowlist it is interactive, otherwise it is read-only

#### Scenario: Guest is refused a non-viewable pane

- **WHEN** a guest attaches a pane not on its view allowlist
- **THEN** the server refuses the WebSocket upgrade (no PTY is spawned) and the client exits with a clear "not shared" message

### Requirement: Raw local terminal with faithful passthrough

While attached the client SHALL put the local terminal into raw mode and passthrough
bytes both directions: local input → the pane, pane output → the local screen,
byte-for-byte (full TUI apps, colors, cursor). It SHALL trap `SIGWINCH` and send the
new size so the remote pane resizes, and it SHALL restore the terminal (cooked mode) on
every exit path (normal detach, error, or signal).

#### Scenario: Interactive TUI works

- **WHEN** the attached pane runs a full-screen TUI (e.g. an editor, or the agent's UI)
- **THEN** it renders and responds correctly, because bytes pass through unparsed to the real local terminal

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

