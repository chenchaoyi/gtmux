# Inspectable transcript activity and Desktop conversation evidence

## Why
Activity previews cannot reveal commands already truncated by the core. Desktop also emits transient hook IDs under a shared backend and stores page navigation as user-role messages.

## What Changes
- Add bounded, optional original tool input and operation metadata without results or status inference.
- Phone/iPad show action icons, ordered previews, one-call detail inspection, horizontal command scrolling, explicit copying and complete group collapse.
- Require a matching rollout for configured-home external Codex rows. Exclude the observed valid Desktop page-navigation envelope from user prompts.

## Surfaces
- terminal: core transcript and external-row evidence change; no new input route or command.
- menubar: consumes corrected core rows and prompts; activity layout remains unchanged.
- phone: two-level activity disclosure, icons and scrollable input.
- iPad: the same shared chat component and interaction, with no separate implementation.
- Web: consumes corrected prompts and rows; existing summary card ignores optional fields compatibly.

## Boundaries
No thread merging by title, cwd, PID or session-tree root. No deletion of native records, logs or follow policy. No remote command execution or new input authority. Other-home evidence cannot be resolved against this home. No new release or running-version replacement in this patch.
