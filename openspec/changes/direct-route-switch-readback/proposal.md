# Keep a Direct route selection stable after a move

## Why

The menu bar calls `gtmux tunnel --server <id>`, then immediately reloads
`gtmux tunnel --servers --json`. The move response has already saved the new URL in
`selftunnel.conf`, but the list's `current` field comes from a separate provisioner read.
If that read still names the old server, the menu bar shows the new route and then jumps
back. A user may repeat a successful move because the selected row looks wrong.

## What changes

- The CLI resolves the current Direct route from the URL this Mac has saved to dial,
  matched against the provisioner's offered list. An unknown or missing local URL falls
  back to the provisioner value. The menu bar, address list and owner route picker share
  that answer.
- The menu bar blocks list refreshes while a move is in flight. Once the move completes,
  it fetches fresh latency measurements using the CLI's resolved current route.
- Regression tests feed a stale provisioner value after a locally persisted move and
  verify the JSON list and phone route metadata. A menu-bar test covers a refresh request
  during a move.

## Surfaces

- **终端 / terminal / attach**: `gtmux tunnel --servers` marks the saved Direct dial route.
  `gtmux attach` is unchanged.
- **菜单栏 / menubar**: selecting a Direct route remains selected through the next refresh.
- **Phone**: the owner route list and address metadata name the same current route as the
  Mac. No phone UI change.
- **iPad**: shares the phone route API and receives the same corrected current marker.
- **Web**: the existing address remains the Web entry point; no route picker changes.

## Scope

This fixes route readback and display. It does not claim that a tunnel is reachable the
instant launchd accepts a restart; reachability still has its own status check.
