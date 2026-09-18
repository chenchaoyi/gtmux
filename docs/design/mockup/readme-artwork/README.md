# README artwork (updated 2026-09-18)

Four artboards: the README's top image in light and dark, and the phone strip in light and
dark. The top image carries all five surfaces at once — the menu-bar popover, a terminal
running `gtmux agents`, the browser workbench, the iPad split view and the iPhone Lock
Screen — because the earlier version showed three and read as a phone app with a terminal
attached.

Every screen here is real except two. The phone and iPad are App Store demo-mode captures.
The browser is the actual page from `internal/server/web`, driven by the screenshot mock.
The terminal is drawn, and its text is what the command prints for the same fleet. The
menu-bar popover is drawn from the app's own measurements: macOS screen capture is
permission-blocked in this environment, and the real popover would show real sessions.

The shipped images are rendered from `docs/assets/screenshots/readme-{hero,screens}.html`,
the same markup at full resolution, by
`GTMUX_ONLY=readme bash docs/assets/screenshots/regenerate.sh`. The images here are the
canvas's small copies. Re-seed with the `/design` helper to view.
