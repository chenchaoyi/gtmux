## ADDED Requirements

### Requirement: Isolated live-clock rendering and session-binding draft retention
`ChatView` SHALL isolate its 1s working clock inside `LiveProgress`, memoize `MarkdownView`, `ToolSteps`, and reading colors, and gate/cache live terminal line parsing. `Composer` SHALL stay mounted across `session_id` resolution on the same pane, preserving text and attachments, migrating stored drafts, flushing dirty drafts on unmount, clearing drafts on send, and protecting held failed text prompts from control keys.

#### Scenario: Typing while session identity resolves and working clock ticks
- **WHEN** the user types and stages an attachment before `session_id` resolves on a working pane
- **THEN** 1s clock ticks skip historical Markdown turns, `session_id` resolution retains the draft and attachments, and control keys keep held failed text prompts
