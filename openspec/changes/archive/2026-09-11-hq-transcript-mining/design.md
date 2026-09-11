# Design — hq-transcript-mining

## D1 — Order the layers by certainty of the judge, not by value

The funnel is a sequence of subtractions, and the first ones are all things the machine
can decide with certainty: tool results (a `tool_result` block), gtmux's own wake lines
and harness blocks (`transcript.ClassifyUserPrompt`, the same classifier the hook uses),
`gtmux send` payloads (the audit journal keeps the head of every one; a typed line whose
normalized head equals one is not the human), compaction summaries and slash wrappers
(flags on the record), pastes (length). Measured on the design machine these remove
>99% of the bytes before any judgement is made. Only the last layer, "is this line a
correction", is a heuristic — and by then the remainder is small enough that a
high-recall / low-precision judge is correct, because HQ reads every survivor.

## D2 — Two signals in v1, chosen by measured precision

Tried on a week of real logs: a lexicon hit on a typed line that follows an assistant
reply reads true ~80% of the time once machine injections are subtracted (before that,
~40%). A normalized error line recurring across sessions is reliable. "Same command
shape failed then succeeded" is noise (every ad-hoc script has the same shape), and
"N consecutive edits to one file" cannot tell iteration from thrash without reading; both
are left out. The lexicon is a bounded list in one file, zh + en, and is documented as
recall-first.

## D3 — Candidates carry the exchange, not a paraphrase

A correction without what it corrects is meaningless ("did you even run it" says nothing
alone). Each candidate is the human line plus the tail of the assistant text it answers.
This also follows the faithfulness finding: what HQ later files should keep the exchange
as the entry's exemplar, because a condensed rule alone is what agents ignore.

## D4 — One spool, one gate; the miner never writes the KB

`internal/mine` is a leaf (imports `transcript`, `events`, `state`). It returns
candidates; `internal/hq` appends them to the pending-distill spool with the existing
`captureCandidate` shape plus additive fields (`source`, `context`, `session`, `project`,
`count`). The distill sensor's spool floor already fires on depth, so mined candidates
pull the next distill forward with no new trigger. HQ drains them with the verbs it has.

## D5 — The ledger is what makes "daily" safe

`~/.local/share/gtmux/mine/`:
- `sources.json` — per log file: byte offset after the last complete line, size, mtime.
  A file whose size shrank below its offset is re-read from 0 (rewritten, not appended).
- `emitted.json` — every candidate id ever emitted (`sha1(session+uuid)[:12]` for a
  correction; the error signature for a recurrence). Never re-emitted, dismissed or not.
- `errors.json` — signature → count, distinct sessions, first/last seen, emitted-at.
  The count keeps growing after emission: that is the "written but still hit" counter.
- `passes.jsonl` — one line per pass: when, files scanned, bytes read, candidates.

Correction candidates are bounded to a window (default 30 days back from the first run;
`--since all` for the stock) because a 1000-line spool is not something HQ drains by
hand. The error tally is over everything scanned, because counts are the point.

## D6 — Where the cadence lives

A `mineSensor` beside `distillSensor` in the slow tick, gated to once per
`hqWake.mineIntervalHours` (default 24; `0` disables), and only when an HQ home exists
(the spool lives there). It never raises a wake of its own: the spool floor does that.
