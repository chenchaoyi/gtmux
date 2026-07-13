# agent-dispatch Specification

## ADDED Requirements

### Requirement: Reclaim a bare pane not in the ledger

`gtmux reap` SHALL reclaim a manually-created window that has no ledger entry, given a
bare pane id: it derives the repo context from the pane's cwd (the enclosing git
worktree + its branch) and applies the SAME safety gate as a ledgered reap — the
worktree must be clean and the branch merged unless `--abandon` overrides — then kills
the window, removes the worktree, and deletes the merged branch. When the gate fails it
reports exactly what blocks it and changes nothing. This closes the gap where
`gtmux reap <pane>` reported "no dispatch" for a hand-made window, leaving it
un-reclaimable.

#### Scenario: A manual window is reclaimed under the safety gate

- **WHEN** `gtmux reap <pane_id>` targets a window with no ledger entry whose worktree
  is clean and branch merged
- **THEN** it reclaims the window/worktree/branch, the same as a ledgered dispatch

#### Scenario: A dirty manual window is report-only

- **WHEN** the bare pane's worktree has uncommitted changes (and no `--abandon`)
- **THEN** it reports the changes and reclaims nothing

### Requirement: Dispatched work is self-describing in tmux

`gtmux spawn` SHALL set the created window/pane title to a task slug so a glance at
tmux reads the fleet. The slug SHALL be derived as: an explicit `--title`, else the
worktree/branch slug, else a normalized head of the goal. One feature maps to one
worktree by convention.

#### Scenario: A spawn names its window

- **WHEN** `gtmux spawn <goal>` creates a window
- **THEN** the window/pane title is the task slug (`--title`, else worktree/branch, else
  goal head)

### Requirement: Headless dispatch for heavy work

`gtmux spawn --headless` SHALL run the dispatched agent detached with NO tmux window —
proxied by construction and tracked via the events/ledger path — so HQ can dispatch
heavy or batch work (a build, a bulk edit) without parking its main input loop or
cluttering tmux with a window the user did not ask to watch.

#### Scenario: Heavy work runs without a window

- **WHEN** `gtmux spawn --headless <goal>` runs
- **THEN** the agent runs detached with no tmux window, and the dispatch is tracked like
  any other
