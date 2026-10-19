## Why

DeepSeek Harness histories are readable, but a DSH conversation controlled by a
community terminal client is still treated as an external history. Users need the
same managed tmux workflow as other harnesses while retaining one DeepSeek Harness
identity and the existing Web-only observation boundary.

## What Changes

- Add a modular terminal-client adapter with dst as the first implementation.
- Verify the current conversation, data home, terminal-client process and tmux attachment
  before attributing events or enabling input; preserve external Web behavior.
- Deliver through a native, session-targeted command with a one-use request and
  explicit acceptance, reusing pane draft/permission safeguards.
- Expose incremental public-message evidence for HQ, reconcile persisted history
  after missed live events, and distinguish consumption from explicit review.
- Extend installation, doctor, transcript documentation and all consumer surfaces
  through the existing core contracts. Use the official DeepSeek identity.

## Capabilities

### Modified Capabilities
- `agent-integration`: verified DSH terminal clients, native command delivery,
  message evidence and installation diagnostics.

## Impact

Agent/client registry, DSH reader and native bridge, hook attribution, radar,
resume/spawn/send, HQ events, installer/doctor and bilingual documentation. No new
model proxy, external input to Web conversations, guest scope expansion, automatic
approval or forced restart of running agent work.

## Surfaces

- terminal: verified DSH pane identity, native input, spawn/resume and message pages.
- menubar: existing radar/chat/focus consumers receive the shared managed pane.
- Phone: existing pane chat/send routes retain authorization and draft protection.
- iPad: same core and phone app contracts; no separate adapter or layout change.
- Web: existing pane mirror receives shared state; native DSH Web conversations stay external and read-only.
