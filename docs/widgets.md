# Usage widgets

On iOS/iPadOS 17 or later, add **Agent usage** from the system widget gallery. Small
and medium Home Screen widgets show quota, balances or recorded output. Lock Screen
widgets support circular, rectangular and inline layouts. Live Activities remain a
separate feature and keep their existing OS support.

Open gtmux once after adding a widget. Pair and select an owner server, then return
to the Home Screen. Tap the widget to open that server's existing usage sheet; HQ
does not need to be running. A guest share cannot populate a usage widget.

**Automatic** shows the current server's most constrained quota first. A medium
widget can show two readings, prioritizing current readings over older ones. Edit the widget to select a particular Agent; the
choices come from actual available readings, including custom providers. If the
selected provider disappears, the widget shows unavailable data rather than silently
switching to a different provider. Switching servers follows the app's current
selection; widgets do not save a second connection.

Quota percentages mean **used**, not remaining. Actual quota and balance readings
retain their units; separate accounts, currencies and wallets are never added
together. Without those readings, each agent can show today's and the last seven
days' output tokens, labelled **Local records**. This covers conversations recorded
by the connected machine, not account-wide usage, a bill or remaining quota. Tokens
are never converted into money. DSH output history can supply this view; a balance
appears only when the server actually obtains it. Balance formatting support does
not establish a working DSH balance integration.

Absent history remains unavailable; an explicitly recorded zero remains zero.
Output keeps the server's recorded day and scan time. After the display's day
changes, cached output is labelled with its source date rather than called today's
output. Historical quota and already-reset windows are excluded from newly
projected data. A cached window reaching reset asks for an update instead of
inventing replenished capacity.

The widget shows the reading time. After 15 minutes it marks the cached information
as an older reading; compact Lock Screen layouts also identify old data. Small widgets
use a compact layout at larger text sizes to keep the Agent, main value and reading
time within their fixed bounds. Opening the usage sheet provides the full details.

## Refresh and privacy

The parent app refreshes periodically once per minute while active, checks again on
foreground entry or a connection change, and requests background refresh no earlier
than 15 minutes later. It does this only when a usage widget is
configured. Background execution and WidgetKit redraws are controlled by the OS;
sleep, low power, force quit, unavailable credentials or an unreachable server can
delay them. This is a dated reading, not a live connection or a billing forecast.

The widget extension reads a bounded display snapshot from
`group.com.gtmux.app.usage`. It contains provider labels, usage windows, recorded output totals/dates, wallet
amounts, a server display name and timestamps. It contains no pairing bearer, server
address, provider account identifier, conversation or session history. The extension
does not include the parent's Keychain reader or network client.

The parent reads its existing private pairing store without changing its protection.
It confirms owner access before requesting usage, rejects HTTP redirects, limits
response size and duration, and discards results after a connection change.
Disconnecting, choosing a guest, removing a pairing or an authorization rejection
clears the shared snapshot. Temporary inability to read protected credentials leaves
an explicitly dated reading. Widget content is marked privacy-sensitive; system
privacy and Lock Screen settings determine when it is concealed.

## macOS and delivery

The snapshot model, formatting and SwiftUI views compile on macOS 14 or later and
are shared for a future native Mac extension. **This change does not install a native
macOS widget.** Apple's iPhone-widget Continuity is a separate system feature and
does not count as a native gtmux Mac extension.

The current Mac app is a directly distributed Developer ID SwiftPM bundle. A native
extension needs its own sandboxed target, embedding/signing/notarization checks and
a supported shared container. Apple's recommended `group.` container arrangement on
macOS requires a provisioning profile. Those changes must be verified against the
Mac packaging workflow before enabling a native extension; installing an unsigned
extension or copying pairing credentials into a shared container is not a substitute.

For the next signed iOS delivery, the app and `GtmuxWidget` identifiers must both
enable the same App Group and use matching provisioning profiles. Export verification also checks that signed App
Group grants are authorized by the embedded profile; older source packages without
a group remain valid. The notification
extension does not join this group. Unsigned simulator compilation does not verify
Developer Portal capability registration, profiles, background scheduling or real
device installation. Existing submitted builds are not changed by this source work.

## Implementation and references

- `shared/UsageWidgets`: display-only model/store/views and a separate parent-only
  host target. Swift tests exercise real loopback HTTP and typed snapshot boundaries.
- `mobileapp/ios/UsageWidgets`: parent refresh coordinator, thin React Native bridge
  and WidgetKit/App Intents configuration. The Xcode target registration helper keeps
  the credential reader and transport out of the extension.
- `mobileapp/src/usage`: pairing lifecycle and the single `gtmux://usage` app handoff.

Presentation research used [Moshi's agent usage documentation](https://getmoshi.app/docs/agents-usages).
Platform behavior follows Apple's [configurable widgets](https://developer.apple.com/documentation/widgetkit/making-a-configurable-widget),
[refresh budgets](https://developer.apple.com/documentation/widgetkit/keeping-a-widget-up-to-date)
and [App Group container guidance](https://developer.apple.com/documentation/xcode/accessing-app-group-containers).
