# Conversation documents and text selection

A document link stays beside the reply that introduced it. Tapping opens a system-style
viewer immediately: filename and Close in the navigation bar, short loading/progress
status, then the document. HTML opens as a static preview. A compact Enable interaction action
asks for native confirmation before running scripts in this file; Use static preview
turns them off again. Info shows the original path and snapshot date. Share exports
the original through the system share sheet. iPhone uses the full screen; iPad uses a
resizable system form sheet with anchored share controls. Errors keep Retry and Copy path
available. No separate document dashboard is added.

## Providing a file

The agent or user runs this on the server after saving a report (use the real transcript
session ID and its home ID, if present, from `gtmux agents --json`):

```sh
gtmux artifact add /absolute/path/report.html --session-id SESSION_ID
# For a conversation with a home identity:
gtmux artifact add /absolute/path/report.html --session-id SESSION_ID --home-id HOME_ID
```

Return the printed Markdown link in the reply. An existing absolute-path Markdown link
also resolves if that exact source path was published for that conversation. Relative
paths remain visible but cannot be resolved without an absolute registered source.
Old file links are not automatically made available. Do not publish credentials,
personal data or files the user did not intend to share with their paired devices.

`gtmux artifact list` prints registered-copy metadata as JSON. `gtmux artifact remove ID`
revokes future downloads and removes only that copy; the source and saved device copies
remain. There is no automatic historical-file scan or global cache cleanup. Copies are
stored in the private state `artifacts/` directory, limited to 256 files / 240 MiB and
30 MiB each. Reaching the limit fails; choose explicit copies to remove.

## Boundaries

Owner/full-scope connections only; guest sharing never inherits document access. The
server only resolves registered identifiers or exact registered source paths in a
session/home identity. It does not expose arbitrary file reads. Sources must be owned,
nonempty regular files; leaf symlinks, hard links, common credential locations and
unsupported extensions are refused. This is defense in depth, not content classification:
the publisher remains responsible for choosing a document safe to provide.

Metadata and content travel through the current authenticated server connection, with
bearers only in headers. The native transport rejects every redirect. Content is an
attachment with `nosniff`, `no-store` and a sandbox CSP, including HTML. The iOS viewer
verifies full byte count and SHA-256 before opening a private temporary copy.

HTML defaults to script-disabled static rendering, with external resources, frames,
workers, forms, normal network requests, new windows and external navigation blocked.
WebKit is ephemeral, has no application bridge and never receives pairing headers.
Only an explicit native confirmation enables scripts for the verified SHA-256 in the
current viewer; request flags cannot enable them. Retry resets consent, a changed file
requires new confirmation, and closing forgets it. Inline charts/buttons and embedded
images that depend on scripts need this interactive mode.

**Interactive mode is trusted content, not a network sandbox.** CSP and WebKit request
rules do not cover every script networking API: a synthetic loopback test confirmed
WebRTC/STUN UDP bypasses these rules. The consent explains that scripts may attempt
network connections. Normal HTTP subresources, external navigation and application
credentials remain blocked, and native Close/Info/Share stay outside the document.
Scripts may consume resources or imitate content inside their preview. Static mode
never runs file scripts, including this WebRTC probe. External-resource reports may be
incomplete in either mode. Camera/microphone/motion permission requests are denied;
element fullscreen is disabled. The public geolocation delegate is denied on iOS 27+;
older iOS uses platform location authorization, which gtmux does not request.
PDF/images use Quick Look with editing and
external links disabled. Text/Markdown uses selectable plain text, limited to its first
512 KiB; sharing preserves the full original. Images have pixel limits before decoding.
Closing cancels work and removes this viewer's UUID cache. A process crash can leave a
private cache directory; no broad deletion is attempted on next launch.

## Selecting text

New iOS builds use one UITextView/TextKit layout for displayed rich text and native
selection. Long press, drag either handle, then Copy. Adjacent prose paragraphs form one
selection scope; code, tables, headings and tool controls are separate. The reply's
existing Copy action still copies the entire Markdown reply. Selection pins only that
text's displayed revision and callback targets, and pauses automatic scroll following;
new agent output continues and appears after selection clears. Width/font-scale changes
are remeasured natively. Bounded collapsed previews keep their previous behavior until
expanded. Older binaries fall back to the existing block-copy wrapper.

Terminal ANSI/theme colors are presentation, not transcript facts. Markdown emphasis is
preserved; arbitrary terminal colors do not imply waiting, success or approval.

## Other platforms and verification

The shared TypeScript reference/metadata/request model contains no iOS-only transport
policy. A future Android adapter can implement `GtmuxDocuments.open` and the native text
manager; it must enforce the same permission, download and isolation contract. Android
currently retains RN text selection and reports previews as unsupported. Browser/menu-bar
native preview is not included. See [API contract](../../api/contract.md).

Go tests cover publication identity, tamper checks, quotas, revocation and authorization.
Jest covers link preservation, rich text/callbacks and selection during streaming. The
standalone fixture in `mobileapp/ios/tests/` compiles the production UIKit components
without real server data; build with `build-conversation-fixture.sh PRIVATE_OUTPUT_DIR`.
Native visual/gesture acceptance and the full React Native build are separate checks.

The native permission/fullscreen policy follows Apple's public
[WKUIDelegate](https://developer.apple.com/documentation/webkit/wkuidelegate) and
[WKPreferences](https://developer.apple.com/documentation/webkit/wkpreferences/iselementfullscreenenabled)
interfaces; no private WebKit preferences are used.
