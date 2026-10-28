# Conversation documents and text selection

A document link stays beside the reply that introduced it. Tapping opens a system-style
viewer immediately: filename and Close in the navigation bar, short loading/progress
status, then the document. HTML opens as a static preview. A compact Enable interaction action
asks for native confirmation before running scripts in this file; Use static preview
turns them off again. Info shows the original path and snapshot date. Share exports
the original through the system share sheet. iPhone uses the full screen; iPad uses a
resizable system form sheet with anchored share controls. Loading and failures use a centered, scrollable status view with a complete localized
explanation and visible actions. Retry is offered for recoverable failures; denied access
returns to the conversation, while an unsupported format can share the verified original.
Unavailable files keep Copy path (or Copy file link). Share appears in the toolbar only
after verification. Status text and buttons wrap at large text sizes; the ready header
puts its controls on separate lines at accessibility sizes. No separate document dashboard is added.

## Providing a file

A local file link in the conversation opens on a paired phone as it does in a local
client: the user clicks once. The server verifies the exact Markdown destination in the
public prompt/reply of that conversation and its home, then snapshots the supported
owned file internally. This includes links in existing conversation history. Code blocks,
inline code, tool arguments and raw HTML do not grant file access. A path request cannot
substitute another file or conversation. Absolute paths and local file URLs work; relative
chat destinations require a verified conversation working directory.

Opening a verified source path reads its current revision. A fixed `gtmux-file:` link
continues to identify immutable bytes. If a temporary source is gone, an existing authorized
snapshot can still be read. Removal records a private version tombstone, so another remote
click cannot silently recreate that removed copy. An explicit local add may provide it again.

The optional command supplies a fixed version, an unlinked file, or compatibility with an
older server:

```sh
gtmux artifact add /absolute/path/report.html --session-id SESSION_ID [--home-id HOME_ID]
```

For tmux use the owning turn's `session_id` from authenticated `/api/transcript`, not the
pane ID or tmux session name. External rows provide `session_id` and, when present, `home_id`.
Return the printed fixed file link when the specific revision matters. Do not include
credentials, personal data or files the user did not intend to provide to paired devices.

`gtmux artifact list` prints private-copy metadata. `gtmux artifact remove ID` removes only
that snapshot and stops future downloads; originals and already downloaded device copies
remain. Storage is bounded to 256 copies / 240 MiB, 30 MiB each; limits do not trigger a
historical scan or deletion of other files.

## Markdown reading

Markdown defaults to formatted, selectable reading: headings, emphasis, lists, block quotes,
fenced code and tables. Tables scroll within the reading column, styles follow system
appearance, and the reading width is capped on iPad. Show source / Show preview switches
views; Share always exports the original Markdown. Text and Markdown read at most the first
512 KiB for display, with the full original retained for sharing.

A Markdown report can display local images and open related files within its directory tree.
The server reads the verified parent snapshot, checks that the requested destination is an
actual Markdown link/image, and binds the result to the same session/home. Declared paths
outside that tree, symlink escapes and foreign or revoked parents fail. The native image
loader uses opaque resource IDs; document content receives no credentials or API bridge.
External images do not load. At most 32 image references, two concurrent downloads, 8 MiB
per image and 20 MiB combined are allowed; dimensions are checked before decoding. Closing,
retrying or switching source cancels only that viewer's resource work and private caches.

## Boundaries

Owner/full-scope connections only; guest sharing never inherits document access. The
server resolves registered copies and verified public conversation links within the exact
session/home identity. Unknown or unlinked paths never expose arbitrary file reads. Sources must be owned,
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
external links disabled. Plain text remains selectable; Markdown uses the formatted preview described above. Display
is limited to the first 512 KiB; sharing preserves the full original. Images have pixel limits before decoding.
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
