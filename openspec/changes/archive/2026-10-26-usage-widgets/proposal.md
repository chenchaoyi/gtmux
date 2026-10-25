# Usage widgets

## Why

Provider capacity is useful before opening a conversation. gtmux currently has a Live Activity but no Home Screen or Lock Screen usage widget. The new surface must retain the existing distinction between quota percentages, account balances and historical readings, and make freshness visible.

## What changes

- Add configurable iOS/iPadOS 17+ Home Screen and Lock Screen usage widgets to the existing extension, preserving Live Activities on their current OS range.
- Share a small typed snapshot and presentation across Apple surfaces. Widgets receive no pairing credentials, host URLs, conversation text or account identifiers.
- Refresh from the parent app using its existing selected-owner Keychain record, foreground lifecycle and budgeted background work. Reject redirects and stale server contexts; clear the snapshot on disconnect, guest selection, unpair or authorization rejection.
- Evaluate and implement native macOS packaging only with a separately compiled sandboxed extension and verifiable signing/container path. Document Continuity separately.
- Add a usage deep link and explain source time, expired windows and unavailable data. No forecast or subscription inference is made from token history.

## Surfaces

- CLI/core: remains the authority; existing usage JSON is the input, without adding billing interpretation.
- macOS menu bar: shared model/views compile on macOS 14+; native extension delivery is deferred until its provisioning, embedding and notarization path is verified. Existing terminal integrations are unchanged.
- iOS/iPadOS: existing widget extension gains Home/Lock families and opens the usage page; app-only refresh retains credential access.
- Web: exploratory tests cover its available conversation/error/interaction paths; no WidgetKit or new usage page support is implied.
- Other platforms: the snapshot contract is independent of iOS navigation; Android remains unsupported until an adapter exists.

## Boundaries

No changes to the submitted Store149 build, access policy, provider sign-in, deployment or installed version. Widget refresh is system-budgeted and cannot promise live updates. Signing/profile prerequisites and unverified device behavior are reported explicitly.
