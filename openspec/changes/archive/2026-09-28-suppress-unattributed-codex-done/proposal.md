# Suppress unowned Codex completion banners

## Why

Codex's shared app-server can emit a Stop that cannot be bound to a tmux pane.
Those events create repeated "Codex / Codex / Finished, tap to jump" banners.
The HQ Codex TUI independently sends "Agent turn complete" through Ghostty.
Its PermissionRequest hook also fires before auto-review decides whether a human
is needed, producing false "needs approval" banners while HQ is working.

## What changes

Keep the lifecycle event in the journal, but suppress the desktop `done` banner
when a Codex Stop has no verified pane. Record a structured suppression reason.
For Codex PermissionRequest, require a persistent live approval menu before
writing a waiting marker or notifying; record suppressed early requests in
diagnostics. Launch Codex HQ with TUI terminal completion notifications disabled, without
changing global Codex or Ghostty settings. Attributed worker completion and
genuine HQ decision notifications remain. Native non-tmux Claude notifications
retain their current behavior.

## Surfaces

- **终端 / terminal**: Codex HQ no longer emits its own Ghostty completion alert after a new launch; global Codex sessions are unchanged.
- **菜单栏 / menubar**: no banner for an unowned Codex completion or an auto-reviewed request without a live approval menu.
- **手机 / phone**: no change; supervisor push suppression already applies.
- **iPad**: same as phone.
- **Web**: no change.
