# HQ state protection, phase 1

## Why
Late reports can reopen completed matters and remove unanswered questions. Event consumption does not retain responsibility to assess a result.

## What changes
Versioned explicit transitions and question resolution, content-only reports, durable reconciliation with fenced leases and atomic assessment receipts. Schema 1 is read compatibly; new writes use schema 2 so old binaries fail closed. No goal/attempt redesign, historical correction, deployment or automatic task execution.

## Impact
Go Store, HQ CLI and periodic ingestion/wake. Existing desktop/mobile/browser presentation is retained; report state becomes a pending proposal with explicit feedback.
