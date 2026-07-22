# remote-terminal-client — delta

## ADDED Requirements

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
