## ADDED Requirements

### Requirement: Direct Codex HQ retains terminal scrollback

A direct interactive Codex HQ launch SHALL share the managed inline-screen default
with worker launch and standard restore, while preserving explicit screen choices,
permissions and notification options. A live HQ SHALL NOT be restarted to apply
this launch default. Shared screen receipt reads SHALL NOT include stale normal
history while the target is in an alternate screen.

#### Scenario: HQ launches with an explicit screen preference

- **WHEN** the configured direct Codex command includes `tui.alternate_screen`
- **THEN** that screen choice remains unchanged and existing permission/notification defaults still apply independently
