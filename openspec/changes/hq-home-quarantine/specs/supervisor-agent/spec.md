# supervisor-agent — delta

## ADDED Requirements

### Requirement: The supervisor identity is precise — the stamp outranks the path

Supervisor-pane resolution SHALL prefer a pane carrying the `@gtmux_hq_home` stamp
(set by `gtmux hq` at spawn) over any pane that merely matches the home by current
or start path; the path criteria SHALL remain solely a fallback for a home with no
stamped pane. The radar SHALL grant `role:"supervisor"` only to the stamped pane
whenever one exists, so a worker session mistakenly parked in the HQ home remains
a normal, visible radar row — never hidden, never woken as the supervisor.

#### Scenario: A worker parked in the home does not steal the identity

- **WHEN** a stamped HQ pane exists and a worker session's cwd is also the HQ home
- **THEN** wakes resolve to the stamped pane and only it carries
  `role:"supervisor"`; the worker lists as a normal agent row

#### Scenario: A legacy home still resolves

- **WHEN** no pane carries the stamp (a home predating it)
- **THEN** the path-based fallback resolves the supervisor as before

### Requirement: The playbook teaches the identity boundary

The seeded playbook SHALL open with an identity self-check — only the session
launched by `gtmux hq` is the supervisor; an agent that received a dispatched task
and finds itself reading the charter was mis-spawned, must not adopt the charter or
spawn anything, and reports for re-dispatch — and its dispatch rules SHALL require
an explicit `--cwd <project dir>` on every spawn. The shipped playbook version
SHALL be bumped so existing homes receive the guard on the next `gtmux hq` after an
update.

#### Scenario: A mis-spawned worker reads the charter

- **WHEN** an agent with a concrete dispatched goal starts inside the HQ home
- **THEN** the charter itself instructs it to decline the supervisor role and
  report for re-dispatching instead of spawning
