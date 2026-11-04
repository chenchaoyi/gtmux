## Why

Codex code-mode records the outer exec call separately from typed native completion
events. The current reader excludes all item_completed events, hiding actual file
changes, parsed file reads/searches and MCP calls. Static source inspection is not
an execution receipt; use the recorded native events instead.

## What Changes

- Admit only explicitly supported native completion types in the streaming reader.
- Parse commands, file changes, MCP input and selected extension metadata without retaining result bodies or reasoning.
- Keep original code-mode calls, deduplicate exact matching recorded call IDs, and preserve append/history/full-input identity.
- Reuse the Go transcript contract and canonical trace/activity model across consumers; add ordered, duplicate, oversized and permission-boundary regressions.

## Surfaces

terminal transcript/API: richer recorded actions. menubar: unchanged. phone/iPad:
existing Chat/Trace consumers show shared operation labels and inspectable file changes.
Web: the same core facts and shared trace model. No new subscriptions, session-follow,
input rights, release/installation or access-policy changes.
