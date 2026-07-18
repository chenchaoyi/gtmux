# Change: draft-detect-excludes-ghost-suggestion

## Why

The draft detector added for stuck-dispatch classification has a **false positive**:
it reads Claude Code's **suggested-next-command ghost text** — the dim autosuggestion
in the composer that needs Tab to accept and is NOT user input — as a genuine
"unsubmitted draft," and fires a false `waiting` (needs-you). Reproduced live on panes
`%85` and `%14`: `%85`'s composer showed `ESC[2m ping %14 that the charter text still
needs coordinating ESC[0m` (SGR 2 = faint), and the detector reported a stuck draft.

Root cause: draft detection runs on a PLAIN capture (`tmux.CaptureFull`, no `-e`), so
the faint SGR that distinguishes ghost text from real input is stripped away — the
dim suggestion is indistinguishable from typed input. CC renders its ghost suggestion
faint (`ESC[2m`), while real user input is normal brightness (and CC's dim *status*
text uses a gray 256-color, `ESC[38;5;246m`, NOT SGR 2), so SGR 2 is a precise signal
for the ghost suggestion.

## What Changes

- **A faint-aware draft extractor.** `dispatch.DraftOfColored(coloredCapture)` strips
  ANSI while DROPPING any text rendered faint (SGR 2), then locates the draft region as
  before. Ghost suggestion → empty draft; real input (bright) → non-empty draft.
- **`tmux.CaptureFullColor`** — a bounded color capture (`capture-pane -e -p -S -200`)
  so the callers can see the rendering.
- **The three "is there ANY user draft?" sites switch to the faint-aware path:**
  - `stuckDispatchKind` (radar `waiting` reclassification) — the reported bug.
  - the `wakeDone` guard (suppresses a `done` when a dispatch holds an unsubmitted draft).
  - the HQ nudge draft-guard (`internal/hqnudge`) — a faint ghost in HQ's own composer
    was HOLDING wakes behind a phantom draft (silent, and arguably worse).
- The delivery verifier (`deliver.go`) is unchanged: it matches a SPECIFIC pasted
  payload (head+tail), which ghost text can never satisfy, so it has no false positive.

## Capabilities

### Modified Capabilities

- `agent-radar` — the unsubmitted-draft-reads-as-waiting rule excludes faint ghost text.
- `agent-dispatch` — the "blocked holding an undelivered draft is never done" rule
  excludes faint ghost text.
- `supervisor-agent` — the HQ half-typed-draft nudge guard excludes faint ghost text.

## Non-goals

- Not changing hook-driven waiting, the delivery verifier, or the startup-gate detection.
- Not a general SGR renderer — it keys narrowly on SGR 2 (faint), the confirmed signal.
