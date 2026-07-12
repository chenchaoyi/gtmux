# Terminal Theme Specification

## Purpose

Make the terminal mirror AUTO-MATCH the user's real host terminal
(Ghostty/iTerm2/default) — resolved server-side and served over the API. The BROWSER
mirror matches colors, 16-color palette, cursor, AND font (a font picker + bundled
fonts). The MOBILE app (native `<Text>` renderer) matches colors, palette, and cursor,
and renders text in the SYSTEM monospace — font-bundling/selection is browser-only;
mobile font SIZE is a local per-pane control. The radar status-language colors are
semantic and are never themed. (The mobile side once bundled fonts + had a picker back
when it used xterm-in-a-webview; that renderer was removed in #346 and the native
renderer never regained font-bundling — the spec below matches today's reality.)

## Requirements


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

### Requirement: Apply the terminal theme on the pane surfaces

Both the mobile native-`<Text>` renderer and the browser mirror SHALL fetch
`/api/theme` and apply it to the pane render — background, foreground, cursor,
selection, and the 16-color palette — replacing the previously hard-coded values. On
the BROWSER mirror, the terminal's `fontFamily` SHALL additionally map to a bundled
font when it matches (else the default bundled font). The MOBILE renderer applies only
the colors/palette/cursor and always uses the system monospace (font-family mapping is
browser-only). The radar status-language colors SHALL NOT be themed.

#### Scenario: Pane matches the terminal's colors

- **WHEN** a paired client opens a pane
- **THEN** the terminal renders in the resolved theme's colors/palette/cursor, while
  the radar status colors are unchanged

#### Scenario: Font mapping is browser-only

- **WHEN** the browser mirror resolves a `fontFamily` in its bundled set
- **THEN** the browser pane renders in that font; the mobile pane renders in the
  system monospace regardless (it does not bundle or select fonts)

### Requirement: Mobile font-size control

The mobile app SHALL provide a local, per-pane font-SIZE control in the Detail toolbar
(stepped A−/A+ over a small preset range), independent of the terminal's point size.
The mobile app does NOT offer a font-family picker or pinch-to-zoom — text always uses
the system monospace.

#### Scenario: Adjust the pane font size

- **WHEN** the user taps A− / A+ in a pane's Detail toolbar
- **THEN** the pane font size steps to the next preset, a local choice that does not
  follow the terminal's point size

### Requirement: Bundled fonts (browser mirror)

The curated monospace fonts SHALL be bundled for the BROWSER mirror (vendored woff2 via
`@font-face`) so it renders them with no network. SF Mono/Menlo SHALL be offered only
via the CSS `ui-monospace` generic (not bundled/redistributed). The mobile app bundles
no terminal fonts — it uses the system monospace — so this is browser-only.

#### Scenario: Bundled font renders offline (browser)

- **WHEN** the browser mirror resolves/uses a bundled font with no network
- **THEN** the browser terminal renders in that font
