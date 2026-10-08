# Guide screenshots: where each one came from

Raw frames are the App Store simulator captures of 2026-10-07 (`shots97/raw-{iphone,ipad}-{en,zh}/`,
iPhone 1320×2868, iPad 2752×2064). To regenerate after a re-shoot, keep the file names:

- iPhone: `sips -s format jpeg -s formatOptions 88 --resampleWidth 900 <raw>.png --out <name>.jpg`
- iPad: crop the bottom 93 px home-indicator grip first (`sips -c 1971 2752 --cropOffset 0 0`), then the same command with `--resampleWidth 1600`

Each line is `<file> ← <raw frame>`; `<l>` is `en` or `zh`, and each language's file comes from that language's frame.

- `watch-a-fleet-radar-<l>.jpg` ← `raw-iphone-<l>/01-radar.png`
- `watch-a-fleet-panes-<l>.jpg` ← `raw-ipad-<l>/03-panes.png` (cropped)
- `hq-supervisor-ipad-<l>.jpg` ← `raw-ipad-<l>/02-hq.png` (cropped)
- `hq-supervisor-your-call-<l>.jpg` ← `raw-iphone-<l>/03-hq.png`
- `hq-supervisor-knowledge-<l>.jpg` ← `raw-ipad-<l>/04-knowledge.png` (cropped)
- `phone-and-web-approval-<l>.jpg` ← `raw-iphone-<l>/02-terminal-approval.png`
- `phone-and-web-usage-<l>.jpg` ← `raw-iphone-<l>/05-usage.png`
- `phone-and-web-servers-<l>.jpg` ← `raw-iphone-<l>/06-servers.png`
