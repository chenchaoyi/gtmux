# Tasks — dispatch-file-channel

One PR. Two independent halves (input channel · re-entrancy) plus the docs/playbook that
make the standard action findable.

## A — the shell-free payload channel

- [x] A1. `internal/dispatch/payload.go`: `ReadPayload(path, stdin)` — read a file or `-`
      (stdin) as bytes, strip at most ONE trailing newline, reject empty/whitespace-only,
      cap the size so `--goal-file /dev/zero` fails instead of hanging.
- [x] A2. `internal/app/spawn.go`: `--goal-file <path|->`; refuse file + positional
      together; refuse an empty result. Usage line updated.
- [x] A3. `internal/app/send.go`: `--message-file <path|->` with the same rules
      (`gtmux send <pane> --message-file f`). Usage line updated.
- [x] A4. `internal/app/oneshot.go`: stage a non-single-line one-shot goal to a file and
      launch `gtmux oneshot-run --goal-file <path>`; the runner reads and unlinks it. A
      short single-line goal keeps the readable inline form.

## B — re-entrancy: converge, don't accumulate

- [x] B1. `internal/dispatch/git.go`: `AddWorktree` returns a `Worktree{Path, Branch,
      Reused, NewBranch}` and REUSES an existing worktree that already serves the branch
      (path occupied by a different branch stays a hard error).
- [x] B2. `internal/dispatch/ledger.go`: `ResumableTask(worktree, session)` — the newest
      undelivered, own-session ledger entry matching this dispatch.
- [x] B3. `internal/app/spawn.go`: adopt a resumable prior attempt (relaunch the agent if
      the pane fell back to a bare shell), and UPDATE its ledger entry instead of adding a
      second one.
- [x] B4. `internal/app/spawn.go`: roll back a worktree/branch this invocation created
      when a later step fails and nothing is resumable.

## C — tests

- [x] C1. `payload_test.go`: byte-exact round-trip for a payload containing backticks,
      `$`, single + double quotes, newlines and CJK; trailing-newline rule; stdin; empty;
      oversize.
- [x] C2. `internal/app/goalfile_test.go`: the parse layer — file wins nothing silently
      (file+positional is an error), `-` reads stdin, and the goal string handed to
      delivery equals the file bytes.
- [x] C3. `deliver`-level round-trip: a fake `IO.Paste` captures what would be pasted;
      assert it equals the original payload byte-for-byte (this is the assertion that the
      delivery path itself does not mangle the text — not merely that the command exited 0).
- [x] C4. `git_test.go`: `AddWorktree` twice for the same branch → second call reports
      `Reused`, no error; a path held by a different branch still errors.
- [x] C5. `ledger_test.go`: `ResumableTask` picks the newest undelivered own-session entry
      and ignores delivered / foreign-worktree / non-own-session ones.

## D — the standard action, written where it is read

- [x] D1. `docs/cli.md`: the file channel on `spawn` + `send`, with the reason and the
      "more than one line or any special character → file channel" rule; the re-entrancy
      guarantee.
- [x] D2. `internal/hq/hq.go`: the playbook dispatch section teaches write-file →
      `--goal-file`; bump `hqPlaybookVersion` 12 → 13 with a changelog line.
- [x] D3. `docs/TROUBLESHOOTING.md`: the backtick-in-a-dispatched-goal incident, its root
      cause, and the must-check.
- [x] D4. `make check` green; `openspec validate --specs --strict` green; sync specs +
      archive this change.
