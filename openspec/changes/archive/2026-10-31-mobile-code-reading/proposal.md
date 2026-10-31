# Mobile code reading

## Why
Trace Preview shows plain code and chat's horizontal drag control can activate native page navigation. Code-reading instructions take space without helping the task.

## What Changes
- Share bounded syntax highlighting and selectable, wrapping code across native tool details and Trace.
- Remove the horizontal drag handle and explanatory information action; retain concise recorded-data provenance.
- Let long chat input open in a dedicated wrapping reader with copy and close.
- Keep raw input and copying exact; adapt colors and layout to light/dark, phone/iPad and Dynamic Type.

## Impact
| Surface | Impact |
| --- | --- |
| CLI/server | No protocol or data changes |
| Menu bar | No change |
| Phone/iPad | Code rendering, reading controls and gesture regression |
| Browser | Existing implementation retained; no new API |
| Website | No change |
