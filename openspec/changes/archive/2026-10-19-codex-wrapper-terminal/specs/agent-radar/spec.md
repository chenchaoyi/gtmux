## ADDED Requirements

### Requirement: Interactive Codex below a generic launcher retains Codex identity

Another registered harness (including a version-named Claude client) SHALL NOT qualify merely because a Codex tool child exists. A launcher-owned tmux pane MAY use the existing Codex hook and exact delivery-binding paths only when a bounded shared verifier finds one Codex descendant with matching pane TTY, foreground process group and terminal stdin, and revalidates the identities. The launcher name SHALL NOT be an allowlist or grant control authority. Existing session, home, payload, draft and ambiguity guards SHALL remain in force. Historical source metadata SHALL remain distinct from current control authority. A pipe-backed app-server, detached worker, background process, changed incarnation or ambiguous subtree SHALL NOT qualify. Old external records SHALL NOT be attached by cwd, age or host-name similarity alone.

#### Scenario: A custom launcher owns an interactive Codex terminal

- **WHEN** a unique Codex descendant is in the pane's foreground group and uses its terminal stdin
- **THEN** the pane can use normal Codex hooks and exact delivery receipts while retaining its launcher separately

#### Scenario: A shared backend was started from tmux

- **WHEN** a Codex backend has piped stdin or no unique foreground terminal ownership
- **THEN** its history remains external and it gains no pane input route

#### Scenario: A source-less old observation remains unbound

- **WHEN** an external record has no verified terminal attachment
- **THEN** no current pane claims it based only on the launching program's name or shared directory
