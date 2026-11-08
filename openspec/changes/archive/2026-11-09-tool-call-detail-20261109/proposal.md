# Recorded native tool details

## Why
Native service-call arguments/results exist in Codex logs, but selected-field parsing drops query arguments and the readers show blank Preview/Raw with a false summary-only warning.

## What Changes
- Load recorded MCP/command arguments/results through the existing scoped exact-record reader.
- Share typed detail interpretation across App Chat/Trace and Web, including explicit empty/error/loading states and paged large bodies.
- Keep transcript polls/cache bounded and retain original call identity/input when native mirrors enrich it.

## Surfaces
- terminal: shared parser gains metadata; no new command/input authority.
- menubar: existing conversation/core consumers ignore additive fields.
- phone: shared typed ToolCallDetails in Chat/Trace.
- iPad: the same RN detail components at wider layouts.
- Web: generated canonical analyzer with browser rendering.

## Boundaries
Existing owner/full-paired authentication, guest denial, session/home resolution and 32MiB record bound remain. No results enter transcript polls/cache. No deployment, release, installed-device or service changes.
