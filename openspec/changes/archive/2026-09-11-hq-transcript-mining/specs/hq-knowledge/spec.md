# hq-knowledge (delta)

## ADDED Requirements

### Requirement: Session transcripts are mined for candidates, LLM-free and incremental

gtmux SHALL run a deterministic miner over the coding agents' session logs on the
machine and SHALL emit candidates into the pending-distill spool, tagged `source:
"transcript"`. The miner SHALL never call a model, SHALL never write a knowledge entry,
and SHALL never read a byte it has already read: a per-file byte watermark and a set of
emitted candidate ids live in a ledger under the state dir.

Before judging a line the miner SHALL subtract what the machine wrote: tool results,
harness-injected blocks, gtmux's own wake lines, `gtmux send` payloads (matched against
the audit journal), compaction summaries, slash-command wrappers, and pastes over a
length bound.

Two candidate kinds SHALL exist in v1: a correction-shaped exchange (a typed human line
following an assistant reply, carrying a correction lexicon hit or emphatic punctuation,
with the tail of the assistant text as context) and a recurring tool error (a normalized
error signature seen in two or more distinct sessions, with its count). Both are leads;
precision is the distill pass's job.

#### Scenario: A `gtmux send` payload is not mistaken for the commander

- **WHEN** a worker's log contains a typed line whose normalized head equals the head of a
  `gtmux:audit:send` record
- **THEN** no candidate is emitted for that line, even if it matches the lexicon

#### Scenario: A pass never emits the same candidate twice

- **WHEN** the miner runs a second time over the same logs
- **THEN** it reads only bytes appended since the recorded offsets and emits no candidate
  whose id is already in the ledger

#### Scenario: A known footgun keeps counting after it was filed

- **WHEN** an error signature already emitted appears in a later session
- **THEN** its count and session set in the ledger grow and no new candidate is emitted

### Requirement: Mined candidates ride the existing spool and cadence

Transcript candidates SHALL use the same spool and the same drain verbs as `gtmux
capture` candidates (`knowledge add --capture <key>` / `dismiss --capture <key>`), with
additive fields (`source`, `context`, `session`, `project`, `count`). The miner SHALL run
from the serve slow tick once per `hqWake.mineIntervalHours` (default 24; `0` disables)
only when an HQ home exists, and SHALL raise no wake of its own — the distill spool floor
is the trigger. `gtmux knowledge mine` SHALL run a pass by hand (`--dry-run` lists without
writing, `--since <dur>|all` bounds the correction window, `--status` prints the ledger).

#### Scenario: The sensor respects its interval

- **WHEN** the slow tick fires less than the interval after the last pass
- **THEN** no logs are opened
