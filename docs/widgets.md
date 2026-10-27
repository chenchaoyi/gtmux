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

## Native Mac widget

On macOS 14 or later, the menu-bar app embeds a native WidgetKit extension. Choose
**gtmux → Agent usage** in the Mac widget gallery, then keep the menu-bar app running.
The native Mac widget is included in gtmux 1.0.152 or later. Update gtmux and
restart the menu-bar app before adding it; 1.0.151 binaries do not include the
extension.
Small and medium widgets use the same display models and views as iPhone/iPad.
They show this Mac's readings; they do not use an iPhone connection or require HQ.
Edit the widget to select Automatic or a provider with available readings.
A tap opens the existing usage page in gtmux.

The parent app checks for a configured native widget before polling the existing
`gtmux usage --json` report. Polls are bounded and at least a minute apart, never
overlap, and explicit timeline reloads are at least five minutes apart. WidgetKit
can defer rendering. When the parent quits or the Mac sleeps, the dated cache remains
and becomes stale; the extension cannot refresh it over the network.

The sandboxed extension receives only a typed display snapshot in the separately
entitled `<signing-team>.com.gtmux.menubar.usage` Mac container. The identifier is
derived from the actual Developer ID signature, not hardcoded or shared with iOS.
Apple supports this team-prefixed macOS group without a provisioning profile.
The CLI has no group entitlement, and the extension has no network or automation
entitlement. The parent keeps its existing nonsandboxed terminal integration.

The normal Mac release workflow uses Xcode to build a thin host around the same
SwiftPM menu-bar library and compile the configuration intent into both app and
extension. This produces App Intents metadata in both bundles; copying an
extension into a manually assembled SwiftPM executable is insufficient. It builds
both architectures, embeds resources and
signs the extension before its parent, and notarizes/staples the complete app.
Ad-hoc builds compile the extension but have no team container; they cannot verify
real sharing. A source build or unsigned CI compilation alone does not prove system
registration, gallery rendering, click routing or notarization. Apple's iPhone-widget
Continuity is a separate system feature and does not count as this native extension.

## iOS delivery

For the next signed iOS delivery, the app and `GtmuxWidget` identifiers must both
enable the same App Group and use matching provisioning profiles. Export verification also checks that signed App
Group grants are authorized by the embedded profile; older source packages without
a group remain valid. The notification
extension does not join this group. Unsigned simulator compilation does not verify
Developer Portal capability registration, profiles, background scheduling or real
device installation. Existing submitted builds are not changed by this source work.

## Implementation and references

- `macapp/Widget`, `WidgetShared`, `UsageWidgetPublisher`: native extension, signer-scoped storage and bounded parent refresh; the committed Xcode project is rebuilt by `Widget/generate-project.rb`.
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
