## Context

The pane mirror renders with xterm.js (mobile: in `react-native-webview`,
`xtermAsset.ts`; browser: `internal/server/web/`). Colors + font are currently
HARD-CODED to one user's Ghostty values. gtmux already detects the host terminal
(`internal/terminal/detect.go`, `terminal.Active()`) and has Ghostty + iTerm2
drivers, but the `Terminal` interface is about CONTROL (FocusTab/IsViewing/…), not
appearance. The CLI must stay **cgo-free**; the radar **status-language colors are
semantic and must never be themed** (only the pane render is themed).

## Goals / Non-Goals

**Goals:** auto-match the user's ACTUAL terminal (Ghostty/iTerm2; default
fallback) with zero config; serve a resolved theme over the API; apply it on both
the mobile + browser terminal; a minimal settings panel (match-toggle, font size,
bundled-font picker). Extensible: a new terminal = one appearance reader.

**Non-Goals (v1):** theme preset gallery, paste/import themes + the 450+ corpus,
`.ttf` import, 16-ANSI editor, `.itermcolors`, Nerd Fonts, real ligature shaping,
light/dark split, cursor/opacity/padding, per-session overrides. Appearance global.

## Decisions

- **Appearance is a SEPARATE reader, not a method on the control `Terminal`
  interface.** New `internal/terminal/appearance.go`: `func Appearance() Theme`
  reuses the existing detection (which terminal is active) and dispatches to a
  per-terminal parser; unknown → `defaultTheme()`. Keeps the control interface
  clean and the radar terminal-agnostic. _Alt:_ add `Appearance()` to `Terminal` —
  rejected: not every driver has a config, and the radar path doesn't want it.

- **`Theme` shape:** `{source string; background, foreground, cursor string;
  palette [16]string; fontFamily string; fontSize float}` — all colors lowercase
  `#rrggbb`. Served as JSON from `GET /api/theme` (authenticated) via a new
  `Deps.Theme func() Theme`. Read per-request (cheap) so config edits show up.

- **Ghostty parser:** read `~/.config/ghostty/config` (and the macOS Application
  Support path) as `key = value`, honoring repeat-appends and `config-file`
  includes. Resolve a named `theme` against the themes corpus
  (`$XDG_CONFIG_HOME/ghostty/themes`, `share/ghostty/themes`) when present — the
  theme file is itself `palette/background/foreground/cursor-color` lines, so the
  SAME parser ingests it; then user keys override (theme-first load order). Honor
  `theme = dark:NAME,light:NAME` by taking the dark side for v1 (no light/dark
  split yet). Map `font-family`/`font-size`.

- **iTerm2 parser:** read `~/Library/Preferences/com.googlecode.iterm2.plist` with
  a **pure-Go plist** lib (`howett.net/plist` — cgo-free, well-used). Pick the
  default profile (`Default Bookmark Guid` → matching entry in `New Bookmarks`,
  else first), read `Ansi 0 Color`…`Ansi 15 Color`, `Background/Foreground/Cursor
  Color` (dicts of `Red/Green/Blue Component` floats 0–1 → hex), and `Normal Font`
  (e.g. `"JetBrainsMono-Regular 13"` → family + size). Vendoring a new dep: vet
  cgo-free + license, add to go.mod.

- **Font mapping + bundling.** The terminal's font name is mapped to a bundled font
  by normalized family (e.g. `JetBrainsMono`/`JetBrains Mono` → JetBrains Mono);
  no match → the default bundled font (still themed colors), surfacing the name in
  settings. Bundle a curated set as latin-subset base64 woff2 via the existing
  `@font-face` mechanism (`gen-xterm-asset.mjs` reads `mobileapp/assets/fonts/*`):
  **System (`ui-monospace`, 0 bytes) + JetBrains Mono + Fira Code + IBM Plex Mono +
  Cascadia + Hack(subset)**, ~100–150KB total. Browser mirror serves the same.

- **Apply + override model.** Default ON: both surfaces `fetch('/api/theme')` on
  connect and apply colors+palette+cursor (+map font). Manual overrides (font
  family, font size, match-off) persist locally (`AsyncStorage` / `localStorage`).
  **Font size is ALWAYS local** (a 13–15pt desktop size doesn't translate to a
  phone) — the terminal's size is at most a hint; the phone keeps its own scale +
  pinch-zoom. Remove the hard-coded values.

## Risks / Trade-offs

- **iTerm2 active-profile ambiguity** → use the default profile; document that
  per-pane profile colors aren't resolved in v1.
- **Ghostty named theme not on disk** (installed oddly) → fall back to whatever
  explicit `background/foreground/palette` keys exist, else default theme.
- **New plist dependency** → must be pure-Go (cgo-free gate) + acceptable license;
  pin it. Mitigation: a tiny, well-scoped parser usage.
- **Font-name → bundled-font fuzziness** → normalize aggressively; safe fallback to
  default font so colors always apply even when the font isn't bundled.
- **Asset size** grows with the font set → subset to latin (Hack today is un-subset
  at ~104KB; subsetting the whole set keeps it ~100–150KB). Nerd Fonts excluded.
- **Theme staleness** (user edits config while watching) → cheap per-request read +
  the phone refetches on app foreground; no push needed in v1.
- **Don't theme the radar** → the `/api/theme` palette feeds ONLY the pane xterm;
  status colors stay the authoritative values.
