# Regenerating the docs screenshots

One command re-renders every user-facing screenshot with generic data (no real
session names, file paths, server name or token cost):

```sh
bash docs/assets/screenshots/regenerate.sh
```

It produces, under `docs/assets/`:

| Image | Used in | How it's made |
|---|---|---|
| `readme-hero.jpg`, `readme-hero-dark.jpg` | README top image (light and dark) | `readme-hero-light.html` / `readme-hero-dark.html`, filled with App Store captures |
| `readme-screens.jpg`, `readme-screens-dark.jpg` | README "What it looks like" | `readme-screens-light.html` / `readme-screens-dark.html`, same captures |
| `screenshot-detail.png` | `docs/phone.md` | real simulator capture (Detail, Terminal) |
| `screenshot-servers.png` | `docs/phone.md` | real simulator capture (connection page) |

The README artwork needs no simulator of its own: it reads the App Store captures in
`mobileapp/.e2e-artifacts/appstore/` (the iPhone radar, lock screen and terminal reply, and
the iPad split view), which the `appstore-shots` e2e writes (`docs/appstore/submit.md`).
Render only the artwork with `GTMUX_ONLY=readme bash docs/assets/screenshots/regenerate.sh`.
Each template draws the terminal window, device frames and dotted backdrop in plain HTML;
the design canvas it came from is under `docs/design/mockup/readme-artwork/`.

## How it works

- **`mock-serve.js`** — a throwaway HTTP server that answers the handful of
  `/api/*` endpoints the iOS app hits (`health`, `agents`, `pane`, `theme`,
  `options`, …) with the generic fixtures defined at the top of the file. Edit
  those to change what the screenshots show. It never touches your real tmux.
- The mobile shots come from the app's own **`GTMUX_SHOTS` e2e harness**
  (`mobileapp/e2e/__tests__/screenshots.test.ts`) pointed at the mock, with a
  generic server name (`GTMUX_SHOTS_NAME`, default `demo-mac`).

## Prerequisites (mobile capture only)

- A **booted iOS simulator** with the app installed. First time / after app
  changes: `cd mobileapp && npm run e2e:build` (or pass `GTMUX_SHOTS_BUILD=1`).
- Appium's xcuitest driver (one-time): `cd mobileapp && npx appium driver install xcuitest`.
- Hardware keyboard off on the sim (see `mobileapp/e2e/README.md`).
- A first WebDriverAgent build can take a few minutes; the harness waits.

Only need the README artwork (no simulator)?

```sh
GTMUX_ONLY=readme bash docs/assets/screenshots/regenerate.sh
```

After running, review with `git status docs/assets/` and commit the PNGs.
