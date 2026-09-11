# Change: hq-transcript-mining

## Why

HQ's knowledge base learns from two inputs today: the moment a lesson surfaces in HQ's
OWN session (the correction→charter loop), and the event delta at distill time. Neither
reaches what the commander says to the worker sessions. Measured on the machine this was
designed on: a `UserPromptSubmit` event carries a median of 7 characters of the prompt
(the audit budget is 200 bytes), so a correction typed into a worker pane is invisible to
HQ unless someone relays it. Meanwhile the agents' transcripts on disk hold every one of
those exchanges, and the highest-value lessons are exactly the ones a single session
cannot see: the same correction made in five different sessions, the same tool error
hit in eight.

Feeding transcripts to an LLM is out of the question by volume (hundreds of MB a week).
But almost all of that is tool output and the machine's own injections. What the human
actually typed is on the order of a megabyte a week, and the part of it that reacts to
something the agent just did is a few dozen lines a day. That is a candidate set, not a
corpus.

Two published results shaped the design. ACE (Agentic Context Engineering, 2025) splits
learning into a Reflector that reads traces and a Curator that merges itemized lessons
with helpful/harmful counters; gtmux's distill already IS the curator, and this change
adds the reflector's input. "LLM Agents Are Not Always Faithful Self-Evolvers" (ICML
2026) finds agents faithfully use RAW trajectories and largely ignore CONDENSED rules —
which is why a candidate here carries the exchange itself, not a paraphrase.

## What Changes

- **A transcript miner (new, `internal/mine`).** LLM-free, deterministic, incremental. It
  reads the agents' session logs from a byte-offset watermark, subtracts what the machine
  injected (harness blocks, gtmux's own wake lines, `gtmux send` payloads matched against
  the audit journal), and emits two kinds of CANDIDATE:
  - a **correction-shaped exchange**: a typed human line that follows an assistant reply
    and carries a correction lexicon hit or emphatic punctuation, with the tail of what the
    agent had just said as context;
  - a **recurring tool error**: a normalized error signature seen in two or more distinct
    sessions, with its count.
  Both are LEADS for HQ to judge, high-recall by design. Precision is HQ's job.
- **The candidates land in the existing pending-distill spool**, tagged `source:
  "transcript"`, so HQ's distill wake drains them through the verbs it already has
  (`knowledge add --capture` / `dismiss --capture`). No second knowledge store.
- **A daily sensor** in the serve slow tick runs the miner (`hqWake.mineIntervalHours`,
  default 24, `0` disables) and a ledger under `~/.local/share/gtmux/mine/` records every
  source offset, every emitted candidate id and the error tally, so nothing is mined
  twice and the count of a known footgun keeps growing after its lesson was written.
- **`gtmux knowledge mine [--dry-run|--since <dur>|all] [--json]`** runs a pass by hand
  and **`--status`** shows the ledger.
- **Playbook v37** teaches HQ what a transcript-sourced candidate is: check the KB first
  (if the lesson exists, the recurrence is the finding), keep the exchange as the entry's
  exemplar, and count.
- **Scope on this machine**: every Claude Code project the agent wrote a log for. Codex
  and gtmux's own opencode transcript are structured as readers behind the same
  interface but are not wired in v1.

## What Does NOT Change

- The KB, its verbs, the promotion exit, the distill cadence and its zero-change gate.
- `gtmux capture` and its spool format (fields are additive, `omitempty`).
- Nothing in the transcript is uploaded or leaves the machine; the spool lives in the HQ
  home like every other candidate.

## Privacy boundary (a rule for this repo, not just this change)

The repository carries the MECHANISM only. Tests read synthetic logs written into a
temp dir; no fixture, doc, or comment quotes a real session, names a real project other
than gtmux itself, or embeds a real path from the machine it was developed on.
