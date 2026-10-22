## Why

The native external-conversation reader repeats the agent name for every segment,
uses a large metadata header and shows only a flat list of tool titles. The mobile
reader already groups replies and offers compact actions and inspectable calls.

## What Changes

Align the native reading hierarchy and interaction semantics with mobile: compact
source header, participant avatars and attribution, one response group per turn,
Markdown, reply/global folding, icon-only copy/share and tool input inspectors.
Reuse native Markdown, official icons, user/HQ avatar geometry, copy feedback and
bounded tail positioning. Keep native light/dark colours and the existing read-only
CLI model; do not embed a second Web client or duplicate backend parsing.

## Surfaces

- Terminal: unchanged public transcript contract; no input or control changes.
- Menu bar: update its native external-conversation reading window.
- Phone: reference semantics only; existing ChatView remains unchanged.
- iPad: existing shared phone ChatView remains unchanged.
- Web: external reader remains unavailable; no new permissions or rendering path.
