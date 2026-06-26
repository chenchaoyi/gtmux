## Why

The mobile + browser terminal currently uses HARD-CODED colors and font (this
session baked the user's Ghostty values — Hack / #17171a — straight into the
asset). That only matches one user's one terminal. gtmux already supports multiple
host terminals (Ghostty, iTerm2 via `terminal.Active()`), so the terminal mirror
should automatically match WHATEVER terminal the user actually runs, with no manual
config — a thing Moshi/Blink/Termius can't do because they have no companion Mac
process. This is the v1/P0 of user-configurable terminal appearance.

## What Changes

- **Per-terminal appearance reader (server).** Reuse the existing host-terminal
  detection and add an `Appearance()` capability per driver that returns a resolved
  Theme `{background, foreground, cursor, palette[16], fontFamily, fontSize}`:
  - **Ghostty** → parse `~/.config/ghostty/config` (key=value; resolve a named
    `theme` against the bundled iterm2-color-schemes ghostty corpus when present;
    honor the `light:,dark:` split and theme-then-override order).
  - **iTerm2** → parse `com.googlecode.iterm2.plist` (default profile's Ansi 0–15 +
    Background/Foreground/Cursor color dicts → hex, and the Normal Font) via a
    **pure-Go plist** library (CLI stays cgo-free).
  - Unknown/undetected terminal → a sensible default theme.
- **New endpoint** `GET /api/theme` → the resolved Theme JSON (`source` =
  `ghostty|iterm2|default`). No change to existing `/api/*` contracts.
- **Both surfaces apply it** (default ON = "match my terminal"): the mobile xterm
  view and the browser mirror fetch `/api/theme` and apply background/foreground/
  cursor/selection + the 16-color palette; the hard-coded values are removed. The
  terminal's `fontFamily` maps to a bundled font when it matches, else the default.
- **Appearance settings panel (mobile, en/zh):** a "Match my terminal" toggle
  (default ON) vs manual; a font-size control (8–20pt stepper/slider) + pinch-zoom;
  a font picker over a small **curated bundled set** — System (`ui-monospace`→SF
  Mono, 0 bytes) + JetBrains Mono + Fira Code + IBM Plex Mono + Cascadia + Hack, all
  latin-subset base64 woff2 (~100–150KB total). Font size is always a local override
  (terminal pt doesn't translate to a phone); colors follow the terminal unless
  overridden.

### Non-goals (v1)

Theme PRESET gallery (P1); paste/import a Ghostty/iTerm2 theme + ingesting the 450+
iterm2-color-schemes corpus (P2); user `.ttf` import (P2); a full 16-ANSI color
editor; `.itermcolors` XML; Nerd Font variants; real xterm.js ligature shaping
(needs a Node-only addon); per-app light/dark split; cursor style/blink; background
opacity/blur/padding; per-session/per-host overrides. Appearance stays **global**.
The radar **status-language colors are semantic and MUST NOT be themed** — theming
applies to the pane render only.

## Capabilities

### New Capabilities
- `terminal-theme`: read the active host terminal's appearance (Ghostty/iTerm2/
  default), serve it over `GET /api/theme`, and apply it to the mobile + browser
  terminal render — plus a mobile appearance settings panel (match-toggle, font
  size, bundled-font picker).

### Modified Capabilities
<!-- none — existing /api/* unchanged; mobile-app/browser-mirror gain behavior via
     the new terminal-theme capability rather than changing their requirements. -->

## Impact

- **Code:** new `Appearance()` per driver in `internal/terminal` (+ a Ghostty
  config parser and an iTerm2 plist reader); `internal/server` (`GET /api/theme`,
  new `Deps.Theme`); `internal/app/serve.go` wiring. A new **pure-Go plist**
  dependency (cgo-free preserved). Mobile: `XtermView.tsx`/`xtermAsset.ts`/
  `gen-xterm-asset.mjs` (apply theme, bundle the font set), `SettingsScreen.tsx`,
  `AppContext`, `client.ts` (fetch `/api/theme`), i18n. Browser: `web/app.js`,
  `web/style.css`, vendored fonts.
- **APIs:** additive `GET /api/theme`; existing contracts unchanged.
- **Dependencies:** one pure-Go plist lib; ~100–150KB of bundled woff2 fonts.
- **Docs:** correct the stale CLAUDE.md/MEMORY note that gtmux already parses the
  Ghostty config — it does not yet; this change builds it.
