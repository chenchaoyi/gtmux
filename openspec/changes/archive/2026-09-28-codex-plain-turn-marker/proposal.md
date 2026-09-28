# Reconcile Codex turns whose start omitted the session ID

## Why

On 2026-09-28, pane `%16` displayed a completed Codex turn at 14:57, while the
phone still showed `working` at 14:59. Its prompt hook had no session ID and its
Stop hook could not establish a pane. The rollout recorded `task_complete`, but
both the Stop resolver and radar fallback required a session ID in the active
marker and left the pane working.

## What changes

- Read plain active markers as live turns, alongside session-named markers.
- Attribute a Codex completion only to a unique live pane bound to the rollout
  that completed after that turn's marker.
- Let radar reconcile the same evidence on its next read if the hook cannot
  establish ownership.
- Keep markers naming another session and completions predating the turn intact.

## Surfaces

- **Terminal / 终端:** `agents --json` and events settle the right pane promptly.
- **Phone / 手机:** the session header receives the corrected status.
- **iPad / 平板:** the same session status is corrected.
- **Menubar / 菜单栏:** the same agent status is corrected.
- **Web / 网页:** the same agent status is corrected.
