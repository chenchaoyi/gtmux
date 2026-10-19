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

Live acceptance: a clean pinned dst 0.16.1 / official DSH 0.2.0-rc.2 temporary
installation displays the real chat UI. The normal verified CLI send reaches the
actual native agent, returns a matching acceptance receipt and persists exactly one
synthetic public message without its wire command. Its review cursor remains -1.
A real draft-preservation check refuses a second message without altering the
synthetic draft. A complete fresh `spawn --agent dsh --headless` in a clean tmux
server directly launches the client, waits for its native receiver and returns
`landed` from the addressed driver receipt.
This exposed and fixed landing-screen launch and unsaved-first-session identity
boundaries. Earlier blank controls used mixed development dependency resolution;
they are not the final acceptance result.

Not covered: real model execution/completion, live mobile/Mac UI acceptance and an
independent review of this patch. The real existing pane was neither restarted nor
sent a probe. Local component/fixture checks remain author evidence.
