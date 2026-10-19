## 1. Native interface prototype
- [x] Verify dst foreground discovery and component admission using isolated fixtures.
- [x] Verify native command invocation receives the exact target agent and queues identified user input without another writer.

## 2. Shared terminal-client contract
- [x] Add modular discovery, attachment validation, process/home/session identity and lifecycle handling.
- [x] Wire unified DSH radar, transcript, event attribution and supported launch/resume behavior without enabling Web control.

## 3. Input and messages
- [x] Add bounded one-use input requests, native target checks, draft/menu guards and acceptance receipts to CLI and HTTP sends.
- [x] Add incremental public-message reading, durable evidence reconciliation and explicit HQ review progress.

## 4. Setup and delivery
- [x] Update installer/doctor through owned provider-specific writers and preserve unrelated configuration.
- [x] Update paired authority/guide/CLI documentation and validate failure cases and supported UI modes.
- [x] Complete required checks, sync the capability spec and archive this change.

## Acceptance

Actual client loading and test-conversation send acceptance are reported separately
from author fixtures and CI. No live production conversation is restarted or used
as a probe. A missing or incompatible native interface remains explicit rather
than silently selecting another conversation or writer.


Author validation: synthetic attachment, delivery, menu/draft/switch/retry, native
request, public-message paging/reconciliation/review, profile ownership, radar
deduplication and resume regressions pass. The installed dst manifest parser and
actual native host admit the component and register its declared command. A native
DSH CommandRuntime prototype queues only the addressed synthetic agent once.

Live acceptance limitation: isolated dst 0.16.1 with official DSH 0.2.0-rc.2 shows
a blank conversation UI, including a bridge-free control. Its static renderer
smoke test succeeds. Therefore pane-to-composer delivery, live TUI queue acceptance,
real model execution and live mobile/Mac UI acceptance are not established. The
real existing pane was neither restarted nor sent a probe. This is author evidence,
not independent review.
