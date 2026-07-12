# Mobile Pane Renderer Specification

## Purpose

Render a tmux pane's live screen on the phone as a READ-ONLY terminal view that
reads like the real terminal — colors, cursor, arbitrary text selection + Copy —
without a webview. After a ~10-PR xterm.js-in-webview saga (WebGL/canvas/DOM
fragility on real iOS), the renderer is native React Native `<Text>` over the
shared ANSI parser, fed by `GET /api/pane` (`capture-pane -e` snapshots). This is
the engine behind the mobile app's "终端/terminal" Detail view.

## Requirements

### Requirement: Native text renderer from capture-pane snapshots

The system SHALL render the pane from `capture-pane -e` snapshots (an
already-resolved flat colored grid, NOT a live VT stream — so no terminal emulator
is needed) using native `<Text>` and the shared ANSI/SGR parser, mapping foreground
+ background, bold/dim, and 256-color / truecolor. It SHALL NOT use a webview or
xterm.js. It SHALL cap rendering to the last N lines of the capture (currently 350)
for scroll performance.

#### Scenario: Colored screen renders

- **WHEN** a pane snapshot with SGR color is shown
- **THEN** the view renders the text with matching colors via native `<Text>`,
  with no webview and no soft-keyboard pop-up on tap

#### Scenario: Long scrollback capped

- **WHEN** a capture returns more than the line cap
- **THEN** only the last N lines are rendered

### Requirement: Long-press selection with a visible highlight

The system SHALL let the user long-press to select arbitrary text and Copy it, with
a VISIBLE selection highlight that keeps the underlying colors readable. Because a
colored, deeply-nested `<Text selectable>` selects+copies but draws no visible
highlight on a real device, selection SHALL ride a separate FLAT, single-color
`<Text selectable>` layer with TRANSPARENT glyphs overlaid on the colored layer, so
the iOS highlight (its own translucent layer) tints the colors behind it — with no
content jump and no mode switch.

#### Scenario: Long-press shows highlight + Copy

- **WHEN** the user long-presses the pane and drags
- **THEN** a translucent highlight appears over the selected colored text and the
  Copy callout is offered, with no layout jump

### Requirement: Freeze the snapshot while touching

The system SHALL freeze the rendered snapshot (text AND cursor) while the user is
touching the pane, so a streaming pane's refresh does not wipe an in-progress
selection or scroll position, and SHALL thaw shortly after the touch ends
(buffering the latest snapshot and applying it on thaw).

#### Scenario: Selection survives a refresh

- **WHEN** the user is holding a selection and a new pane snapshot arrives
- **THEN** the on-screen snapshot stays frozen until shortly after the touch ends,
  then updates to the latest content

### Requirement: Follow the live bottom

The system SHALL follow the bottom of a live pane by default (auto-scroll to the
newest content), stop following once the user scrolls up to read history, and
resume following — reaching the true live tail — when the user scrolls back to the
bottom, even while the pane is actively streaming.

#### Scenario: Scroll up then back to bottom

- **WHEN** the user scrolls up in a streaming pane and later scrolls back down to
  the bottom
- **THEN** the view reaches the live tail and resumes auto-following new output

### Requirement: Cursor cell and glyph normalization

The system SHALL draw the pane's text cursor as a reverse-video cell positioned
from the bottom-anchored `cursor{x,up,visible}` of `GET /api/pane`, and SHALL
normalize glyphs that render wrong on iOS (e.g. map U+23FA "⏺" emoji-presentation
to U+25CF "●") so tool-call markers and similar glyphs render as intended.

#### Scenario: Cursor lands on the prompt row

- **WHEN** a pane reports a visible cursor
- **THEN** a reverse-video cell is drawn at that column on the bottom-anchored row

#### Scenario: Record glyph normalized

- **WHEN** a snapshot contains U+23FA
- **THEN** it is rendered as U+25CF (no emoji presentation)
