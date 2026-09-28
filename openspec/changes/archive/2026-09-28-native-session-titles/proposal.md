# Show saved titles for native agent sessions

## Why

A Codex session running outside tmux has a saved conversation title, but the radar
does not put it in `task`. Every client therefore falls back to its working directory
or agent name. In the reported case, a session named “调查 SpringBoard 崩溃问题” appeared
as “gtmux”, making three unrelated Codex sessions hard to distinguish.

## What changes

Read Codex's `session_index.jsonl` by exact session ID and populate `task` for native
rows when a saved `thread_name` exists. Read the bounded recent index once per radar
poll, use the last entry for a renamed session, and keep the existing fallback when
the title is missing. A transcript prompt or database prompt is never substituted as
a title. Native session state, identity, deduplication, and adopt rules do not change.
Other agents keep their current fallback until a verified title source is established.

## Surfaces

- **Terminal (including attach)**: `gtmux agents --json` now carries the native
  Codex title in `task`; attach remains a tmux-only terminal connection.
- **menubar**: existing native row title priority (`task` before project) displays it.
- **Phone**: existing radar row title priority displays it; the row stays read-only.
- **iPad**: shares the phone radar implementation and receives the same title.
- **Web**: existing radar `primary()` also prefers `task`; verify the code path,
  without claiming device/browser visual acceptance.
