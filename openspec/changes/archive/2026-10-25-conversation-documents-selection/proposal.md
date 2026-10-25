## Why

Conversation replies lose local report destinations, and iOS Copy only selects a whole
rendered block. A paired device should open a deliberately provided document and select
just the text the user needs, without leaving the conversation.

## What Changes

Add a bounded local publication registry, owner-only metadata/download endpoints, a
platform-neutral preview contract and an isolated iOS document adapter. Use one native
TextKit renderer for rich prose and adjustable selection, preserving existing full-reply
Copy and older-client fallbacks. Keep terminal theme colors separate from message state.

## Surfaces

- CLI / terminal: `artifact add/list/remove`; only an explicit local command publishes a copy.
- Core/API: bounded immutable copies, exact conversation identity, owner-only downloads.
- Mobile phone / iPad: native preview, range handles and responsive system presentation.
- macOS menubar: existing conversation rendering; no new native preview in this change.
- Web/future Android: shared reference/metadata contract; existing text selection remains;
  native document adapters are future work, with a clear unsupported message today.

## Security and compatibility

No remote arbitrary-path reader. A file link alone does not publish its target. Guest
links cannot read documents. Downloads never execute on the server's origin. HTML runs
only offline in an ephemeral WebKit view with no application bridge. Publication is a
snapshot; removal revokes future reads, not copies already saved by a device.

## Follow-up verification

The original offline-script assumption was too broad: a synthetic loopback test showed
WebRTC/STUN UDP bypasses CSP and WebKit request rules. The current capability spec and
design therefore default HTML to script-disabled static preview. Interactive scripts
require native confirmation for the verified file hash in this viewer and explicitly
do not promise universal network isolation. Request credentials/bridges remain absent.
