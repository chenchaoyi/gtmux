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

### Requirement: Headless dispatch for background heavy work

`gtmux spawn --headless` SHALL dispatch heavy or batch work (a build, a bulk edit)
WITHOUT popping a terminal tab, and SHALL mark its window as background so a glance at
tmux distinguishes it from windows the user should watch — while keeping the dispatch
fully proxied, land-verified, tracked, and reapable (its pane still exists; "headless"
means no terminal tab and out of the way, not untracked). This lets HQ offload heavy
work without parking its main input loop.

#### Scenario: Heavy work runs without a terminal tab

- **WHEN** `gtmux spawn --headless <goal>` runs
- **THEN** no terminal tab is opened, the window is marked background, and the dispatch
  is proxied, verified, tracked, and reapable like any other
