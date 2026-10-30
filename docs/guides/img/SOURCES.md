# Guide screenshots: where each one came from

Raw frames are the website simulator captures of 2026-10-08 (`site1539/{en,zh,ipad-en,ipad-zh}/`,
iPhone 1320×2868, iPad 2752×2064), made by `mobileapp/e2e/__tests__/site-shots.test.ts`
with demo data and real agent marks. Both the app and the simulator system language
match each set. To regenerate after a re-shoot, keep the file names:

- iPhone: `convert <raw>.png -resize 900x -quality 88 <name>.jpg`
- iPad: crop the bottom 93 px (home-indicator grip and iPadOS resize corner), then resize:
  `convert <raw>.png -crop 2752x1971+0+0 +repage -resize 1600x -quality 88 <name>.jpg`

Each line is `<file> ← <raw frame>`; `<l>` is `en` or `zh`, and each language's file comes from that language's frame.

- `watch-a-fleet-radar-<l>.jpg` ← `<l>/01-radar.png`
- `watch-a-fleet-panes-<l>.jpg` ← `ipad-<l>/03-panes.png` (cropped)
- `hq-supervisor-ipad-<l>.jpg` ← `ipad-<l>/02-hq.png` (cropped)
- `hq-supervisor-your-call-<l>.jpg` ← `<l>/03-hq.png`
- `hq-supervisor-knowledge-<l>.jpg` ← `ipad-<l>/04-knowledge.png` (cropped)
- `phone-and-web-approval-<l>.jpg` ← `<l>/02-terminal-approval.png`
- `phone-and-web-usage-<l>.jpg` ← `<l>/05-usage.png`
- `phone-and-web-servers-<l>.jpg` ← `<l>/06-servers.png`

## Native menu-bar captures (2026-10-08)

- `knowledge-menubar-<l>.png`: the current SwiftUI `MenuView`, HQ card expanded.
- `phone-and-web-share-<l>.png`: the current SwiftUI `NewShareSheet`.

Rendered with `NSHostingView.cacheDisplay` from the native app at source commit
`e35465523e74934466b2c696720191ce164c0334`, in light mode with each matching app language.
These are native view captures with generic fixtures, not captures of the operator's
live sessions: three panes (`hq`, `api`, `worker`), five knowledge entries and two pending
promotions. The knowledge image captures only the header and HQ card; the share image
shows the permission picker before a link is created. No real code or credential is used.

## Onboarding terminal and Web captures (2026-10-30)

- `terminal-agents-en.png`, `terminal-agents-zh.png`: the real `agentsTable(readmeFleet())`
  renderer from `internal/app/agents.go` and its existing test fixture at source
  `9f891a8368ca`. A task-local Go test wrote the ANSI output; xterm from the embedded
  Web client rendered it at 116 columns, Menlo 16px, in Chromium (1200×365). These are
  terminal renderings of verified CLI output, not native terminal-window captures.
  `TestREADMEAgentsSampleIsReal` also checks both accessible README text versions.
- `web-workspace-en.png`, `web-workspace-zh.png`: the actual embedded Web assets at
  `d182d8eb` (including the merged section-label fix), served by `tests/web/fixture.cjs` on a random loopback port.
  Chromium used en-US / zh-CN respectively, at 1280×850. The synthetic enrollment
  and sessions belong only to the fixture. No running gtmux, user pairing, agent
  history or native test lane was used.

- `menubar-agents-en.png`, `menubar-agents-zh.png`: current real `MenuView` rendered
  through `NSHostingView.cacheDisplay` in an isolated SwiftPM capture test at the same
  source. Three synthetic agents, isolated defaults, and a `GTMUX_BIN` fixture stub
  prevent live session reads or actions. Capture bounds include the header, session
  rows and local workspace action; the XCTest-host version footer is excluded.

The overview's iPhone and iPad frames remain the corresponding real simulator captures above.
Their captions identify the example data/date and avoid teaching superseded font
buttons. The five overview sections each have a distinct surface image. Reviewed
against current command help, `MenuView.swift`, mobile i18n and the Web fixture on
2026-10-30. These captures do not establish physical-device or notification delivery.
