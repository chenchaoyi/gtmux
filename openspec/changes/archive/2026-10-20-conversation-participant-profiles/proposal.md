## Why

Reply folding is detached from its content and only works locally after a global
collapse. Message avatars cannot explain the actual speaker. Profiles must use
each harness's native facts rather than imposing Codex metadata on every agent.

## What Changes

- Attach per-turn copy/share/fold controls to the reply and retain a two-line preview.
- Open an accessible participant dialog from human, agent and sender avatars.
- Add an owner-only read API over scoped, bounded native metadata readers and
  native-hook observations, with sanitized execution evidence.
- Keep agent-specific model, role, subscription, permission and reasoning semantics.

## Non-goals

No model calls, takeover authority, arbitrary executable probes, user status-line
replacement, new radar states, automatic release or access-policy changes.

## Surfaces

- terminal: lifecycle and control unchanged; optional native observers collect display facts.
- menubar: existing radar unchanged; no participant dialog in this iteration.
- phone: attached per-turn controls and participant profile dialogs.
- iPad: same dialogs adapt to available height/width and retain the split conversation.
- Web: existing diagnostic transcript unchanged; shared owner API available to future UI.
