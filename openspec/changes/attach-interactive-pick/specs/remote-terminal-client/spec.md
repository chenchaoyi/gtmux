# remote-terminal-client — delta

## MODIFIED Requirements

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
