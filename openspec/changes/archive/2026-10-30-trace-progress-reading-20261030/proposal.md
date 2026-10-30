# Read trace progress without losing position

## Why
The supplied iPhone screenshot526dbc9c shows a redundant Follow latest switch, oldest-first entry, repeated metadata and no view of current progress. Readers need to reach the latest recorded activity immediately and inspect history without refresh moving it.

## What Changes
- Implicit tail-following based on reading position, with a one-shot latest action only when away; no follow mode toggle.
- Group native records by stable conversation/turn identity, collapse historical turns and repeated tool batches, show a concise latest-progress summary and an ordered step rail.
- Keep full-text search, recorded-input details, frozen selected revisions and profile/file routes; virtualize the resulting timeline.
- Align browser default/latest navigation and protect its selected text and scroll anchor.

## Additional user feedback in this iteration
- Align Web session status badges, secondary summaries and persistent section disclosures with the App.
- Share human identity/quotation and recorded fact ordering; use App profile hierarchy, historical identity guards and on-demand execution ancestry.
- Group browser settings and make chat size and exact terminal size immediate/persistent; show extra input tools only for primary touch input.
- Repair missing Codex freeform calls in the shared structural reader and parser while retaining structured-call compatibility. Synthetic regression and in-memory source counts verify the defect without saving private conversation content.

## Surfaces
- Terminal / attach: browser terminal typography only; no input, binding or attach protocol changes.
- menubar: unchanged native UI; shared Codex transcript adapter also restores its public calls.
- phone: native trace progress layout and implicit latest navigation.
- iPad: same components, content-width adaptive details and large text.
- Web: latest/refresh behavior, App-aligned lists/profiles/settings/touch input tools; generated shared trace and profile presentation assets.

## Research
Langfuse session replay and aggregated repeated observations: https://langfuse.com/docs/observability/features/agent-graphs ; Chainlit steps: https://docs.chainlit.io/concepts/step . OpenCode scroll defects show why user scroll position must outrank stale frame corrections: https://github.com/anomalyco/opencode/issues/17996 . Browser control unavailable in this session, so these are official documentation/source references, not live competitor acceptance.
