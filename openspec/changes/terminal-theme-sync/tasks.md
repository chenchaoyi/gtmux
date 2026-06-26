## 1. Server — appearance reader

- [x] 1.1 Add a pure-Go plist dependency (cgo-free; vet license) to `go.mod`; confirm `CGO_ENABLED=0 go build ./cmd/gtmux` still passes.
- [x] 1.2 `internal/terminal/appearance.go`: define `Theme{Source, Background, Foreground, Cursor string; Palette [16]string; FontFamily string; FontSize float64}` and `func Appearance() Theme` that reuses detection and dispatches per terminal; `defaultTheme()` fallback. Unit-testable.
- [x] 1.3 Ghostty reader: parse `~/.config/ghostty/config` (+ macOS App Support path) as `key = value` with repeat-append + `config-file` includes; resolve a named `theme` against the themes corpus (same parser ingests the theme file); user keys override; take the dark side of `light:,dark:`; map `font-family`/`font-size`. Table tests on sample configs.
- [x] 1.4 iTerm2 reader: parse `com.googlecode.iterm2.plist` default profile (Default Bookmark Guid → profile, else first); `Ansi 0..15 Color` + Background/Foreground/Cursor `{Red/Green/Blue Component}` floats → `#rrggbb`; `Normal Font` → family+size. Table test on a sample plist fixture.

## 2. Server — serve the theme

- [x] 2.1 Add `Deps.Theme func() Theme` and `GET /api/theme` (authenticated) returning the resolved Theme JSON; read per-request. Confirm `/api/*` contract unchanged.
- [x] 2.2 Wire `deps.Theme = terminal.Appearance` in `internal/app/serve.go` (and the tunnel's in-process radar).
- [x] 2.3 Test (`webui_test.go`/a new test): `/api/theme` returns valid JSON and stays token-guarded; `make check` green.

## 3. Fonts — curate + bundle

- [ ] 3.1 Acquire + latin-SUBSET the curated woff2 set into `mobileapp/assets/fonts/`: JetBrains Mono, Fira Code, IBM Plex Mono, Cascadia Code, Hack (re-subset, currently ~104KB) — regular + bold; keep license files. Target total ~100–150KB.
- [ ] 3.2 `gen-xterm-asset.mjs`: emit `@font-face` for each bundled family (base64 woff2); "System" maps to CSS `ui-monospace` (no bytes). Regen; validate the bootstrap JS.
- [ ] 3.3 Vendor the same fonts for the browser mirror (or reuse the generated asset's faces); `web/style.css` exposes the families.

## 4. Mobile — apply theme + settings

- [x] 4.1 `client.ts`: `theme()` → `GET /api/theme`. `AppContext`: store theme + appearance prefs (matchTerminal default true, fontFamily, fontIdx) in AsyncStorage.
- [x] 4.2 `XtermView.tsx`/`gen-xterm-asset.mjs`: accept a theme + font prop; apply background/foreground/cursor/selection + 16-palette + cursor decoration via `gtmuxConfig`; map fontFamily→bundled or default. REMOVE the hard-coded Ghostty values.
- [ ] 4.3 `SettingsScreen.tsx` (en+zh): "Match my terminal" toggle (default ON), font-size control (8–20pt) + keep pinch-zoom, bundled-font picker; persist; show the matched source/font.
- [ ] 4.4 Gate: `tsc --noEmit` + `eslint .` clean.

## 5. Browser — apply theme + fonts

- [x] 5.1 `web/app.js`: fetch `/api/theme` on connect; apply colors+palette+cursor to the xterm theme; map fontFamily→bundled/default. Remove hard-coded Ghostty values.
- [ ] 5.2 (Optional v1) a minimal in-page control for font/size; otherwise follow the terminal + a sane default.

## 6. Verify (manual — sim/browser; device for the font set)

- [ ] 6.1 Browser (Playwright): a Ghostty user's pane renders in their resolved theme; an iTerm2 fixture/profile renders its colors; unknown → default. Font from the set applies.
- [ ] 6.2 Mobile (sim): pane matches the terminal by default; settings toggle off → pick font+size persists; bundled fonts render offline.
- [ ] 6.3 Confirm the radar status colors are unchanged (not themed).

## 7. Ship + docs

- [ ] 7.1 Branch → PR → CI green → squash-merge (never main).
- [ ] 7.2 Correct the CLAUDE.md / MEMORY note claiming gtmux already parses the Ghostty config; update [[mobile-terminal-renderer-strategy]] with the shipped theme-sync.
- [ ] 7.3 `openspec` sync + archive; `validate --specs --strict` passes.
