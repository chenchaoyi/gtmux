# transcript-render-bounds

## Why

Switching a long-running session to Chat crashes the phone app. Measured on the
reporter's own gtmux session (`%22`), which is exactly the case they named:

| | |
|---|---|
| `GET /api/transcript` payload | **1.76 MB** |
| turns | 147 |
| reply bubbles (segments) | 1,885 |
| tool-step rows | 2,974 |
| markdown lines across bubbles | 6,374 |
| ⇒ native text nodes, floor | **~9,800** (higher in practice — markdown splits each line further into bold/code/link spans) |
| plus | 1,885 agent avatars |

All of it is parsed at once and rendered at once: `ChatView` maps every turn into a
plain `<ScrollView>` with no virtualization, and `collapsedAll` initializes to `false`,
so every reply is expanded on arrival — the heaviest possible render is the default. (Its
own comment says "`collapsedAll` is the default", which the code contradicts.) A 1.76 MB
`JSON.parse` on Hermes plus tens of thousands of views is a memory spike iOS answers by
killing the app, which is what the user sees as a crash on switching to Chat.

The existing guard bounds the wrong dimension. `maxTranscriptTurns = 300` is documented
as "set high enough not to truncate" — and 147 turns, well under it, still produced
1.76 MB. What hurts a phone is bytes and node count, not turns; a chatty session with 40
long turns is heavier than a terse one with 200.

## What Changes

Two independent bounds, because either alone leaves the failure reachable:

- **The payload is bounded by SIZE, newest-first.** The server keeps the most recent
  turns that fit a byte budget and drops older ones, instead of trusting a turn count to
  imply a size. The response says how many turns were dropped, so a client can tell the
  user its history is truncated rather than silently showing a partial conversation as if
  it were the whole one.
- **The client renders a WINDOW of the newest turns**, not the whole payload, with an
  explicit control to reach further back. Even a bounded payload is thousands of nodes,
  and the tail is what a phone reader actually looks at; older turns cost nothing until
  asked for.

`collapsedAll`'s initial value is left alone: collapsing by default would hide the reply
you opened Chat to read. The window is the right lever — fewer turns fully rendered beats
every turn rendered stubbed.

## Impact

- Specs: `chat-transcript` (payload bound), `mobile-chat-view` (render window).
- Code: `internal/app/serve.go` (the bound), `mobileapp/src/ui/ChatView.tsx` (the window).
- Benefits the web mirror too: it reads the same endpoint.
- Not changed: the parser, the incremental tail cache, and the turn shape on the wire —
  a truncated payload is the same array of the same turns, just fewer of them.
