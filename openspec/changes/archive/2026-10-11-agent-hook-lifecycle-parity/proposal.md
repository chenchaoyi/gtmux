# Agent hook lifecycle parity

## Why

An interrupted Codex turn can remain working because gtmux registers Stop but not Interrupt, and its tmux rollout reconciliation ignores turn_aborted. The same audit found missing Kimi lifecycle events, Cursor response/completion confusion, and doctor coverage limited to three installers. A polling working-to-idle edge also sends a false completion notification after cancellation or failure.

## What Changes

- Audit the eight hook-equipped agents against current primary documentation and record sources and limits.
- Add supported state-related events and distinguish interruption, failure and normal completion throughout the hook, native and polling paths.
- Keep session ownership and newer-turn guards for Codex reconciliation.
- Check and repair every supported hook installer in doctor, including outdated plugins, while preserving foreign configuration and explaining trust/restart requirements.
- Support Kiro's current hook format alongside existing CLI 2.x profiles.

## Surfaces

- terminal: accurate radar state and doctor upgrades.
- menubar: consumes corrected core state; no UI redesign.
- Phone: consumes corrected core state and receives no successful-completion push for interruption.
- iPad: same core state and push behavior as phone.
- Web: same radar data; no false completion SSE alert after interruption.

## Impact

Hook installers, hook classifier/runtime, radar reconciliation, server transitions, agent-integration and env-doctor specs, bilingual integration documentation. No release, deployment or live-session restart is part of this change.
