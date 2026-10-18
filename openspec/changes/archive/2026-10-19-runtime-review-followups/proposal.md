# Bound retained runtime work and align relay contracts

## Why

The remaining user-supplied DSH review identifies retained transcript caches without a process budget, serve announcing credentials before a successful bind, Go relay contract drift, and hot reads that materialize data before trimming it. Notification backlog and validator diagnostics have separate bounded fixes. Reproduce each claim against current main; the three earlier security items are already delivered in v1.0.126.

## What changes

- Bound the existing transcript cache by retained bytes, entry count and inactivity without changing returned histories or parser identity/incremental guarantees.
- Hold a kernel-backed per-user/port serve lease and bind before creating or advertising pairing credentials/status. Tie SSE work to the listener lifetime.
- Align Go relay APNs errors, Live Activity requests and supported wire fields with the Worker using shared contract fixtures.
- Keep the slow-tick watermark cheap without changing retained-sequence semantics; bound recent HQ event reads and git subprocess capture before materialization.
- Let the menu bar reuse the running serve's shared producer snapshot, retaining fresh standalone CLI fallback and the configured cadence.
- Bound the owned desktop notification queue and classify OpenSpec tool failures separately from invalid specs.

## Scope and decisions

No access-policy changes, agent restarts, log/backup cleanup, automatic branch/worktree pruning, or adoption of a fixed release schedule. A high file/branch count is not evidence that another worker's data can be deleted. Large documentation/doctor extractions require demonstrated ownership boundaries rather than a line-count-only rewrite. Already superseded or explicitly deprioritized proposals are evaluated separately from delivered defects.

## Validation

Use disposable homes, synthetic histories/gateways and private listeners. Add regressions that fail on the current paths, preserve source files and wire output, and cover replacement/append/eviction, competing serve starts, APNs rejections/Live Activity parity, bounded output, concurrent notifications and validator startup failures. Run repository gates after implementation; public CI does not prove physical device or push acceptance.

## Surfaces

- CLI: retained cache/event/diff bounds and listener ownership; standalone reads remain fresh.
- Menu bar: shares serve radar with the existing cadence and CLI fallback; bounded notification backlog.
- Phone: unchanged API shapes; Go self-host relay now preserves APNs and Live Activity contracts.
- iPad: same unchanged mobile consumer and relay behavior as Phone.
- Web: unchanged API shapes; shares the bounded server producer and read paths.
