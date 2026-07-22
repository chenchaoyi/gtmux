# resume-conversation-home

## Why

`gtmux restore` relaunched an agent conversation with `cd <recorded-cwd> && claude
--resume <id>`, using the cwd we last saw the pane in. But an agent files its transcript
under the directory the session STARTED in, and it can `cd` elsewhere mid-session — so
the recorded cwd drifts to the moved-to directory and the resume fails with
"No conversation found with session ID". Observed on a real reboot: a session filed under
`/Users/…/niushaofeng` was resumed from `/Users/…/niushaofeng/saas-research` and errored.

A second failure appeared alongside it: a resume record whose conversation no longer
exists on disk at all. There is no directory that would work, yet restore still typed a
`--resume` that could only print an error into the user's fresh pane.

## What Changes

- Before resuming, resolve the record against the agent's own transcript store: for
  Claude Code, locate `~/.claude/projects/*/<session-id>.jsonl` and take the session's
  authoritative `cwd` from the transcript. Resume from THAT directory.
  (The transcript's recorded cwd is read rather than decoding the project-directory name,
  because that encoding is lossy — a directory literally named `a-b` and the path `a/b`
  both encode to `-a-b`.)
- If no transcript exists for the session id, treat the conversation as gone and SKIP the
  resume (logged to the restore log) instead of typing a doomed command.
- Agents whose store we can't inspect are unchanged (no-op, still resumed as before).

## Impact

- Spec: `session-restore` (a requirement for resuming from the conversation's own home).
- Code: new `internal/resume/resolve.go` (`Resolve`), called from
  `internal/app/agent_resume.go` before building the command. `resume.Command` stays pure.
- Back-compat: only affects Claude records; a correct record resolves to the same cwd.
