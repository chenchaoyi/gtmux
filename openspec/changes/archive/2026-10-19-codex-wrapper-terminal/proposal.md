# Manage interactive Codex clients beneath generic launchers

## Why

Radar already recognizes Codex below an arbitrary launcher, while hook and delivery binding paths require the tmux foreground command to be literally `codex`. A launcher can therefore appear as a Codex pane without receiving its conversation hooks or acquiring an exact delivery binding.

## What changes

Use a shared terminal-client verifier for launcher-owned panes: one actual Codex descendant must share the pane's controlling terminal and foreground process group, and its stdin must be that terminal. Reject background, piped, ambiguous, stale or changed processes. Keep existing session/home/payload checks and draft protections. Preserve Codex identity and generic launcher metadata; do not register private launcher brands as agents or infer identity from a display name.

## Surfaces

- terminal / attach: existing Codex input, draft and receipt paths after verified terminal identity.
- menubar: existing Codex pane and source projection; no new agent category.
- phone: existing Codex pane controls; external histories retain their existing policy.
- iPad: same contract as phone.
- Web: same core identity and input gates.

Old external records without process/session attachment proof remain observations. Shared app-server/API clients need a dedicated addressing adapter; simply being descended from tmux does not grant input.
