## ADDED Requirements

### Requirement: Resolve the active terminal's appearance

The system SHALL resolve a Theme — `{source, background, foreground, cursor,
palette[16], fontFamily, fontSize}` (colors as `#rrggbb`) — from the user's ACTIVE
host terminal, reusing the existing terminal detection, with a per-terminal reader:
Ghostty (parse `~/.config/ghostty/config`, resolving a named `theme` against the
themes corpus when present, user keys overriding) and iTerm2 (parse
`com.googlecode.iterm2.plist`'s default profile via a pure-Go plist parser). An
undetected or unsupported terminal SHALL yield a sensible default theme. The CLI
SHALL remain cgo-free.

#### Scenario: Ghostty config resolved

- **WHEN** the active terminal is Ghostty with a `~/.config/ghostty/config`
- **THEN** the resolved Theme reflects its background/foreground/cursor/palette and
  font-family/size, with `source = "ghostty"`

#### Scenario: iTerm2 default profile resolved

- **WHEN** the active terminal is iTerm2
- **THEN** the resolved Theme reflects the default profile's Ansi 0–15 +
  background/foreground/cursor colors and Normal Font, with `source = "iterm2"`

#### Scenario: Unknown terminal falls back

- **WHEN** no supported terminal is detected (or its config is unreadable)
- **THEN** a default Theme is returned with `source = "default"` and no error

### Requirement: Serve the resolved theme

The system SHALL expose `GET /api/theme` (authenticated, like the other `/api/*`)
returning the resolved Theme as JSON, read per-request so config edits are
reflected. Existing `/api/*` contracts SHALL be unchanged.

#### Scenario: Theme endpoint returns JSON

- **WHEN** an authenticated client GETs `/api/theme`
- **THEN** it receives the resolved Theme JSON (`source`, colors, palette, font)

### Requirement: Apply the terminal theme on the mobile and browser surfaces

Both the mobile xterm view and the browser mirror SHALL, by default ("match my
terminal" ON), fetch `/api/theme` and apply it to the pane render — background,
foreground, cursor, selection, and the 16-color palette — replacing the previously
hard-coded values. The terminal's `fontFamily` SHALL map to a bundled font when it
matches; otherwise the default bundled font is used (colors still apply). The radar
status-language colors SHALL NOT be themed.

#### Scenario: Pane matches the terminal by default

- **WHEN** a paired client opens a pane with "match my terminal" on
- **THEN** the terminal renders in the resolved theme's colors/palette (and the
  mapped font when bundled), while the radar status colors are unchanged

#### Scenario: Unbundled font falls back, colors still apply

- **WHEN** the resolved `fontFamily` is not in the bundled set
- **THEN** the pane uses the default bundled font but still applies the theme colors

### Requirement: Appearance settings panel (mobile)

The mobile app SHALL provide an appearance settings panel (en+zh) with: a "Match my
terminal" toggle (default ON) vs. manual; a font-size control (8–20pt) plus
pinch-to-zoom; and a font picker over a curated BUNDLED set (System via
`ui-monospace` plus several latin-subset woff2 monospace fonts). Font size SHALL
always be a local override; manual selections SHALL persist locally.

#### Scenario: Turn off matching and pick a font + size

- **WHEN** the user turns "Match my terminal" off and picks a bundled font + size
- **THEN** the pane renders in the chosen font/size and the choice persists across
  launches

#### Scenario: Font size is always local

- **WHEN** "Match my terminal" is ON
- **THEN** colors follow the terminal but the font SIZE follows the local control /
  pinch, not the terminal's point size

### Requirement: Bundled fonts work offline

The curated monospace fonts SHALL be bundled (base64 woff2 via `@font-face` in the
generated asset for mobile; vendored for the browser) so they render with no
network and on phones that don't have them installed. SF Mono/Menlo SHALL be
offered only via the CSS `ui-monospace` generic (not bundled/redistributed).

#### Scenario: Bundled font renders offline

- **WHEN** a bundled font is selected with no network
- **THEN** the terminal renders in that font
