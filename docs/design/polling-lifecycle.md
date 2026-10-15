# Periodic read lifecycle

## Ownership

A read lane belongs to one resource and its current page/client/credential scope.
Independent resources remain independent. Interval ticks skip a busy lane; they do
not queue more work. Mutations retain their own receipts and are never retried or
coalesced by these helpers.

- Phone/iPad HQ overview, live capture, transcript and Detail capture/transcript/options
  use `state/readTask.ts`. Disposing an effect aborts its read and retires its result.
  A successful Detail send invalidates any capture requested before its receipt.
- Browser periodic radar, panes, terminal, chat, options and workbench tiles use read
  lanes in `app.js`. Leaving a view, changing a tile mode or removing a tile cancels
  its lane. Existing view/credential/reply-order fences remain in force.
- Native radar, resource, HQ reader/card, awake and tab-order polls each have a gate.
  Their CLI reads use `capturePoll`; user-triggered mutations use their existing path.

## Bounds and failure semantics

Mobile/browser lanes have a 15-second deadline covering the full read and body parse.
Cancellation is sent to the transport; a transport ignoring it still cannot publish a
late result or keep the lane occupied. Mobile uses the same failure handling as a read
error; browser timeout marks the current connection unavailable. A subsequent tick can
recover. Approval choices are cleared on read failure rather than kept actionable.

Native periodic CLI commands have a 15-second bound per command and a 4 MiB bound per
output stream. Output growth is checked while the process runs. On timeout/output
failure, a verified separate process group receives TERM then KILL so descendants do
not accumulate. No signal is sent to the app's own group. A multi-command poll has a
finite sum of these per-command bounds.

A transient failure is not an empty fleet, knowledge base, history or usage reading.
Keep the last successfully decoded snapshot; a successful empty result may clear it.
HTTP 5xx transcript/digest/event reads throw rather than return an empty list. Owner
and guest authorization boundaries are unchanged.

## Verification boundary

Regression tests use delayed promises and controlled subprocesses to cover admission,
timeout recovery, disposal, invalidation, stale replies, output limits and descendant
cleanup. Consumer tests exercise real HQ/Detail polling and AgentStore. Mutation checks
remove admission protection and must turn those tests red. Browser fixtures use the
real assets and synthetic HTTP responses, never real panes. These checks do not prove
real-device layout, VoiceOver or production network latency.
