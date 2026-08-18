# native-agent-sessions (delta)

## ADDED Requirements

### Requirement: A pane-less hook is proven native before it is treated as native

`$TMUX_PANE` is the hook's first signal for which pane it belongs to, but it is no longer
the only one. An agent may run its conversation in a process that does not inherit the
terminal's environment — Claude Code's background session host (`claude daemon` →
`--bg-pty-host`) is one such process, and it has neither a tty nor `$TMUX_PANE` while its
ancestry still leads back to the tmux pane the user is sitting in.

When `$TMUX_PANE` is absent, the system SHALL attempt to identify the pane from the hook
process's ancestry before recording a native session: it SHALL walk a bounded number of
ancestors and match them against tmux's own pane table, by `pane_pid` first and then by
`pane_tty`. A single unambiguous match SHALL be treated as that pane, exactly as if
`$TMUX_PANE` had named it. Ambiguity or no match SHALL fall through to the existing
native-session path, which stays the default for genuinely non-tmux agents.

The lookup SHALL be bounded and SHALL fail open: it runs only when `$TMUX_PANE` is
absent, walks at most a fixed number of ancestors, and any error resolves to "not
identified" rather than blocking the hook.

#### Scenario: A background-hosted session is still its pane's session

- **WHEN** a hook fires from a process with no `$TMUX_PANE` whose ancestry includes the
  process tmux reports as a pane's `pane_pid`
- **THEN** the event is recorded against that pane — its state, its resume binding, its
  event stream — and no native session record is written

#### Scenario: Only the tty survives the chain

- **WHEN** no ancestor pid matches a `pane_pid`, but exactly one pane's `pane_tty` matches
  an ancestor's tty
- **THEN** that pane is used

#### Scenario: A genuinely native agent is unaffected

- **WHEN** a hook fires with no `$TMUX_PANE` and no ancestor matches any pane
- **THEN** the session is recorded as a `source: "native"` row, as before

#### Scenario: Ambiguity is not a guess

- **WHEN** the ancestry matches more than one pane
- **THEN** no pane is claimed and the native path is taken
