# Show the selected Direct route in both Mac windows

## Why

The two windows now share the access pickers, but the picker called “Route”
chooses Standard or Direct while only Pair your phone shows the actual server
route. A user sees the same setting described two ways and cannot change the
server from Preferences.

## What changes

Call Standard / Direct “Connection method” in both windows. Reserve “Route”
for the named Direct servers. Show the same server list and move confirmation
in Preferences and Pair your phone, backed by one observable store so a move
updates both windows immediately. Keep the CLI as the state source and refresh
measurements after a move. On the paired phone and iPad, show the current route
clearly in Settings and on the route page; refresh Settings when returning from
a move and mark an accepted move while reconnection is in progress.

## Surfaces

- **终端 / terminal**: no change; the CLI remains the source of server state.
- **菜单栏 / menubar**: both windows show Connection method and, for Direct, Route.
- **手机 / phone**: show the selected route and refresh it after a move.
- **iPad**: the shared Settings and Route screens receive the same change.
- **Web**: no change; guests cannot change the host's remote access settings.
