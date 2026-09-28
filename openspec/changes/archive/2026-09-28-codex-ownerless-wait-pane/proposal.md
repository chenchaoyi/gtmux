# Bind ownerless Codex approvals to evidence

## Why

A Codex `PermissionRequest` with neither session ID nor cwd inherited the shared
app-server's first `TMUX_PANE`. On 2026-09-28, HQ approvals at `%21` produced
`Waiting(permission)` records for an idle website pane `%16`, so the phone asked
the user to answer a session that had no question.

## What changes

- Resolve a Codex approval hook without cwd only from a unique session binding.
  Otherwise leave the event pane-less and sense the live menu in its own pane.
- Clear an existing false wait when a quiet Codex pane shows its ready composer.
- Report a live Codex approval menu from that pane and persist its wait on the
  slow tick when the hook could not identify its owner.
- Keep the event in the journal even when its pane cannot be established.

## Surfaces

- **Terminal / 终端:** `agents` and `events` stop attributing an ownerless approval
  to the inherited pane; an existing false waiting row settles to idle.
- **Phone / 手机:** the HQ call and session header consume the corrected status.
- **iPad / 平板:** the same shared status is corrected.
- **Menubar / 菜单栏:** the same shared status is corrected.
- **Web / 网页:** the same shared status is corrected.
