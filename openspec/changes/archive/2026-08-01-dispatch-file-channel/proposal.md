# dispatch-file-channel — a goal channel that cannot be mangled, and a dispatch that can be re-run

## Why

**The interface makes the caller responsible for something the caller cannot guarantee.**

`gtmux spawn <goal…>` and `gtmux send <pane> <text…>` take their payload as ARGV. A
caller — HQ, a script, a human — therefore has to produce a shell-safe rendering of an
arbitrary natural-language instruction. On 2026-08-01 HQ dispatched a Chinese goal
containing code identifiers wrapped in backticks, inside double quotes. The shell did
what double quotes mean: it ran the backticked text as a command substitution, failed
with `command substitution: syntax error near unexpected token 'done'`, and the spawn
never happened.

The instructive part is not the failure, it is the recurrence: **that exact footgun was
already recorded twice in HQ's own knowledge base, and HQ had generalised it into a rule
that same morning** — then broke it hours later. So this is not a memory problem to be
solved with a better reminder. It is a design problem: any sufficiently long natural-
language goal will eventually contain a backtick, a `$`, a quote, or a newline, and a
payload passed as argv must pass through a shell that assigns those characters meaning.
"Be careful each time" is not a property a system can hold.

Two more failures share the root:

- **Truncation / swallowed Enter on long CJK sends.** Already hardened at the delivery
  layer (draft confirm before Enter), but the SOURCE side still hands the text through
  argv, so a caller can mangle it before delivery ever sees it.
- **The failed spawn left a half-built world.** The worktree and branch existed, the
  session did not. Re-running the same command hit `exit status 128` (`git worktree add`
  onto an existing path), and two attempts left two empty sessions with no goal in them.
  The failure path neither rolled back nor converged — every retry made the mess bigger,
  and a human had to clean it up.

## What changes

### ① A payload channel that never meets a shell

- `gtmux spawn --goal-file <path>` and `gtmux send <pane> --message-file <path>`, both
  accepting `-` for stdin. The caller writes bytes to a file; gtmux reads bytes; the
  bytes reach the agent's input box unmodified. There is no shell on that path (delivery
  already stages through `tmux load-buffer -` from a pipe, which is byte-exact).
- Exactly ONE normalization is applied and it is specified: at most one trailing newline
  is stripped, because every heredoc and `printf` ends with one. Everything else —
  backticks, `$`, quotes, interior newlines, CJK — arrives verbatim.
- Passing both a file and positional text is an ERROR, not a silent precedence rule.
- `--oneshot` is covered too: its goal used to be shell-quoted onto the launch line AND
  whitespace-collapsed, so newlines silently died. It now stages the goal to a file that
  the in-pane runner reads (`gtmux oneshot-run --goal-file`), so a multi-line goal
  survives the one-shot path as well.
- The positional form stays for short instructions. The DOCS state the rule plainly:
  more than one line, or any special character → use the file channel.

### ② A dispatch that converges when you re-run it

- `dispatch.AddWorktree` becomes idempotent: an existing worktree already serving that
  branch is REUSED (reported as reused) instead of failing with 128. A path occupied by a
  different branch is still a hard error — adopting someone else's tree silently would be
  worse than the 128.
- **Rollback of this invocation's own side effects.** If spawn creates a worktree and a
  later step fails, the worktree it created is removed and the branch it created is
  deleted, so a failure that cannot be resumed at least leaves nothing behind.
- **Resume of a survivable prior attempt.** Before creating a session, spawn looks for a
  ledger entry from a previous attempt at the same dispatch (same worktree path, or same
  derived session name) that is `delivered:false`, still has a LIVE pane, and owns its
  session. It adopts that pane instead of minting a second session, re-launches the agent
  if the pane fell back to a bare shell, delivers the goal, and UPDATES the existing
  ledger entry rather than adding a duplicate.
- Net effect: **re-running the identical command converges on the correct state** —
  one worktree, one session, one ledger entry, goal delivered — instead of accumulating
  empty sessions.

### ③ The standard action, written down where it is read

- `docs/cli.md` gains the recommended posture with the REASON (argv necessarily traverses
  a shell), not just the flag.
- The HQ playbook's dispatch section teaches write-file → `--goal-file` as the standard
  action for any goal that is not a short single line. `hqPlaybookVersion` 12 → 13 so
  existing HQ homes actually receive it.

## Impact

- **Affected specs:** `agent-dispatch` (the input channel, the one-shot argv exception,
  re-entrancy/rollback) and `supervisor-agent` (the playbook's dispatch standard action).
- **Playbook version:** 12 → 13.
- **No new command.** Two new flags on two existing commands, plus a flag on the hidden
  `oneshot-run` plumbing command.
- **Backwards compatible.** Positional goals/text keep working exactly as they do today.

## Non-goals

- Not removing the tmux paste-buffer from DELIVERY. An interactive agent TUI has no other
  input surface; that path is already byte-exact (`load-buffer -` from a pipe) and
  land-verified. What this change removes is the SHELL between the caller and gtmux.
- Not a general "resume any dispatch" feature. Resume applies to a spawn's own
  never-delivered prior attempt, identified from the ledger — not to adopting arbitrary
  sessions.
- Not changing `POST /api/send` (a JSON body already carries bytes safely).
