# attach-interactive-pick

## Why

When `gtmux attach <host>` is given no `%pane` and more than one pane is attachable,
the client prints the list and **exits** (code 2). The user then has to eye-scan the
list, copy a `%pane` id, and re-run the whole command — a jarring dead-end mid-flow.
Every other pick-one-of-N CLI (Claude's numbered menus, `git add -p`, fzf) instead
lets you choose in place. attach should too.

## What Changes

- When multiple panes are attachable, no `%pane` was given, **and stdin is a TTY**,
  present a **numbered menu** (1..N, one row per pane with session · agent · status ·
  task), read a choice, and attach to it immediately — no re-run.
- Enter selects the default (row 1); `q` / `Esc` / EOF cancels cleanly (no attach).
- **Non-TTY (pipe / script / CI) keeps the current behavior**: print the list to
  stderr and exit non-zero, so automation never hangs waiting on a prompt.
- Single attachable pane still auto-selects (unchanged); zero panes still errors
  (unchanged).

## Impact

- Affected spec: `remote-terminal-client` (pane-resolution behavior).
- Affected code: `internal/connect/run.go` (`pickPane`), a small TTY-gated interactive
  prompt; pure helpers (row formatting + choice parsing) factored out for tests.
- Docs: `docs/cli.md` `gtmux attach` section; the CLAUDE.md attach description already
  says "`[%pane]`" so no command-list change.
- Back-compat: fully additive — the non-interactive path (scripts) is byte-identical.
