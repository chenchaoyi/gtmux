# agent-dispatch — delta

## ADDED Requirements

### Requirement: Spawn never places a worker in the HQ home

`gtmux spawn` SHALL refuse to run a worker in the HQ home — the directory whose
seeded `AGENTS.md` is the supervisor charter — because an agent launched there
reads the charter and impersonates the supervisor (including dispatching further
workers). The refusal SHALL cover every route in: an explicit `--cwd` naming the
home, the inherited caller cwd when `--cwd` is absent (the exact trap when HQ
itself dispatches), and `--pane` reuse of a pane whose current path is the home.
The comparison SHALL be symlink-normalized, and the refusal message SHALL name the
fix (pass `--cwd <project dir>`).

#### Scenario: HQ dispatches without --cwd from its own pane

- **WHEN** `gtmux spawn <goal>` runs with no `--cwd` while the caller's working
  directory is the HQ home
- **THEN** the spawn is refused with a message telling the caller to pass
  `--cwd <project dir>`, and no session is created

#### Scenario: A project dispatch is unaffected

- **WHEN** `gtmux spawn --cwd <project dir> <goal>` names a normal project
- **THEN** the spawn proceeds exactly as before
