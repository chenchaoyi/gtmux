# Change: hq-transcript-mining-all-agents

## Why

`hq-transcript-mining` shipped reading Claude Code logs only. The commander runs Codex,
Kimi Code and opencode sessions on the same machine, and a correction typed into any of
them is exactly as invisible to HQ as one typed into a Claude pane was.

## What Changes

- **One conversation model, four readers.** The miner's judgement (machine subtraction,
  paste bound, lexicon, error signature, carry-over across passes) moves into a shared
  state machine; each agent's reader only translates its envelope into "who spoke, what
  tool ran, what it printed".
- **Codex**: reads `response_item` messages (recent Codex versions no longer write the
  `event_msg` user/agent stream the chat view reads), drops user-role records that open
  with Codex's own XML-ish tags, tallies shell (`exec_command`/`exec`) outputs, unwrapping
  the JSON envelope some outputs carry. Candidate ids use the record ordinal.
- **opencode**: the transcript gtmux writes (`octrans/`); correction leads only, no
  project (the file carries none).
- **Kimi Code**: `context.append_message` (user, non-injection origins) and
  `context.append_loop_event` text parts; correction leads only — no tool-result record
  was observed on a real journal, so none is claimed.
- Default roots honour `$CODEX_HOME` and `$KIMI_CODE_HOME` like the transcript package.

## Privacy

As before: tests write synthetic logs into a temp dir; nothing from a real session enters
the repo.
