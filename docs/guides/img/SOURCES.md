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
