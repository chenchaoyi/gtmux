# Remote session restoration

## Why
After a server reboot an owner away from the computer cannot recover saved tmux sessions from the App. The menu restore action immediately closes its progress surface. Repeated taps and connection loss must not launch the same restoration twice.

## What Changes
- Owner-only preview, explicit durable restore job, and read-only progress API.
- App server action with preview, confirmation and resumable progress.
- Missing saved sessions only, headless on remote invocation, existing strict agent identity and liveness gates retained.
- A process-owned execution lease across CLI/menu/remote and honest partial/pending results.
- Restore remains visible in the menu until explicit dismissal.

## Surfaces
- Terminal: existing interactive restore remains; internal job execution uses the same safe producer, never a client-supplied shell command.
- Menu bar: keep progress visible, honor explicit Escape/outside close, do not steal focus.
- Phone: paired owner can preview/start/check remote recovery; guests cannot.
- iPad: same API and scrollable sheet, adapting to wide/large-text layouts. Android shares this RN flow.
- Web: no new restore control in this change; authenticated owner API has the same scope enforcement regardless of transport.

## Impact
Restore coordination, server API, shared RN server UI, native menu closing policy and documentation. No real user sessions, access policy, signing or deployment are changed by implementation tests.
