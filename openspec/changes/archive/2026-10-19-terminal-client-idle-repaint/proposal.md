# Prefer native terminal lifecycle during UI repaint

## Why

A verified DeepSeek Harness terminal client can repaint its idle UI without a user request. Radar currently treats those frame changes as working, then idle again when the short frame window expires. This creates false working/completion edges even while the generic finished-time cache and session binding are unchanged. A terminal session can have no durable history yet, so history lookup alone cannot fix this case.

## What changes

Declare lifecycle observation as a terminal adapter capability. Only a hook whose native sender matches a freshly verified attachment may publish a bounded private snapshot. Bind it to the harness/home/session/pane/client incarnation and foreground epoch; reject stale turns and late callbacks. Keep this proof separate from inferred active/finished marker caches.

For a verified lifecycle-aware client, native start/question/resume/end events determine status. UI/CPU activity and generic marker heuristics cannot override that source. A later explicit boundary from the exact durable DSH session can repair a missed hook through the existing incremental catalog. With neither source, report neutral running and no fabricated finish time. Current leases still determine ownership; status evidence grants no input authority. Other integrations keep their existing paths.

## Surfaces

- terminal: the shared radar follows native lifecycle phases through repaint, or stays neutral when evidence is missing.
- menubar: consumes the corrected core status; no UI or status-language change.
- phone: consumes the same status and avoids false completion edges; existing UI stays compatible.
- iPad: uses the same core status through the shared phone/iPad implementation.
- Web: consumes the same corrected status, including narrow radar and workbench directory.

## Evidence boundary

Read-only observation of the selected pane found repeated frame-driven edges while generic markers stayed fixed. Its durable history was unavailable. Code inspection found that radar itself also writes the finished-time cache: that cache is not independent native evidence. No transcript corroboration or native completion is claimed for the old session. The correction requires separate verified native proof; existing sessions without it remain neutral until a natural event or explicit durable boundary arrives.
