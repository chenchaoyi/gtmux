# mobile-pane-renderer — delta

## ADDED Requirements

### Requirement: Predictive local echo for typed input

The mobile terminal SHALL show the user's own printable keystrokes and backspaces
IMMEDIATELY as predicted local echo, so typing does not wait for the server round-trip,
while never misleading the user. A predicted character SHALL be drawn at the known cursor
column in a visually distinct **unconfirmed** style (underlined/dimmed) and SHALL be
dropped as soon as the authoritative screen (the `/api/send` response or the next
`capture-pane` poll) confirms or contradicts it — the server screen is always
authoritative. Predictions are local display only; the actual keystroke SHALL still be
sent via `POST /api/send` unchanged.

Prediction SHALL be gated for honesty: it applies only when the measured send round-trip
exceeds a small threshold (a fast link shows no predictions); it SHALL NOT predict when
the cursor is hidden (alt-screen TUI, `cursor.visible=false`) nor for a guest read-only
pane; and any state-changing key (Enter, ESC, arrows, Ctrl-C, Tab, a 1/2/3 approval)
SHALL end the prediction epoch — clearing outstanding predictions and pausing prediction
until the server confirms again. There SHALL be no server or contract change.

#### Scenario: Typing at a prompt over a slow link

- **WHEN** the send round-trip is high and the user types a printable character at a visible cursor
- **THEN** the character appears immediately at the cursor in the unconfirmed (underlined) style, and is redrawn as normal text once the next authoritative screen confirms it

#### Scenario: A wrong prediction is corrected, not trusted

- **WHEN** an outstanding predicted character does not match the authoritative screen when it arrives
- **THEN** the prediction is dropped and the server screen is shown — the mispredicted cell is never left as if real

#### Scenario: Fast link shows no predictions

- **WHEN** the measured round-trip is below the threshold
- **THEN** no predicted echo is drawn (the confirmed screen is shown as-is), so there is no flicker or risk on a fast connection

#### Scenario: A state-changing key ends the epoch

- **WHEN** the user presses Enter/ESC/an arrow/Ctrl-C or taps a 1/2/3 approval
- **THEN** all outstanding predictions are cleared and prediction pauses until the next confirmed screen, so the renderer never predicts across an unpredictable screen change
