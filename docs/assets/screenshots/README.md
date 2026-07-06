# Regenerating the docs screenshots

One command re-captures every user-facing screenshot with **generic** data — no
real session names, file paths, server name, or token cost:

```sh
bash docs/assets/screenshots/regenerate.sh
```

It produces, under `docs/assets/`:

| Image | Surface | How it's made |
|---|---|---|
| `surface-cli.png` | README hero — CLI | rendered from `cli-hero.html` |
| `surface-menubar.png` | README hero — menu bar | rendered from `menubar-hero.html` (version injected from the latest git tag) |
| `surface-mobile.png` | README hero — mobile | a **real** simulator radar capture framed by `mobile-hero.html` |
| `screenshot-detail.png` | `docs/phone.md` | real simulator capture (Detail → Terminal) |
| `screenshot-servers.png` | `docs/phone.md` | real simulator capture (connection page) |

## How it works

- **`mock-serve.js`** — a throwaway HTTP server that answers the handful of
  `/api/*` endpoints the iOS app hits (`health`, `agents`, `pane`, `theme`,
  `options`, …) with the generic fixtures defined at the top of the file. Edit
  those to change what the screenshots show. It never touches your real tmux.
- The mobile shots come from the app's own **`GTMUX_SHOTS` e2e harness**
  (`mobileapp/e2e/__tests__/screenshots.test.ts`) pointed at the mock, with a
  generic server name (`GTMUX_SHOTS_NAME`, default `demo-mac`).
- The three **hero** images are self-contained HTML rendered to PNG by headless
  Chrome (612×760), then downscaled. No screen capture of the real menu-bar app
  (macOS blocks that under TCC) — so the menu-bar hero always reflects the
  *current* design here, not whatever happens to be running.

## Prerequisites (mobile capture only)

- A **booted iOS simulator** with the app installed. First time / after app
  changes: `cd mobileapp && npm run e2e:build` (or pass `GTMUX_SHOTS_BUILD=1`).
- Appium's xcuitest driver (one-time): `cd mobileapp && npx appium driver install xcuitest`.
- Hardware keyboard off on the sim (see `mobileapp/e2e/README.md`).
- A first WebDriverAgent build can take a few minutes; the harness waits.

Only need the hero images (no simulator)? Reuse the last captures:

```sh
GTMUX_SKIP_CAPTURE=1 bash docs/assets/screenshots/regenerate.sh
```

After running, review with `git status docs/assets/` and commit the PNGs.
