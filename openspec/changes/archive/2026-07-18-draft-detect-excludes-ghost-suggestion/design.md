# Design: draft-detect-excludes-ghost-suggestion

## Root cause (confirmed with a live capture)

`%85`'s composer, captured with `-e`:

```
ESC[39m<prompt-glyph> ESC[2mping %14 that the charter text still needs coordinating ESC[0m
```

The ghost suggestion is wrapped `ESC[2m` … `ESC[0m` — SGR 2 (faint). On a plain
`CaptureFull` (no `-e`) those markers are gone, so `SplitInputRegion` returns
`"ping %14 that…"` as the draft and every "is there a non-empty draft?" caller reads a
stuck/unsubmitted draft that isn't there.

CC's rendering vocabulary (from the captures):
- **ghost suggestion** → `ESC[2m` (SGR 2, faint). ← the thing to exclude
- **dim status/hint text** → `ESC[38;5;246m` / `ESC[38;5;244m` (a gray 256-COLOR, not
  SGR 2). Not in the draft region anyway.
- **real user input** → normal brightness (no SGR 2).

So SGR 2 within the draft ⟺ ghost suggestion. User-typed input never carries SGR 2
(the user types characters; CC renders them normally), and a pasted dispatch goal is
likewise bright — so dropping faint spans cannot erase real input.

## Decisions

### D1 — Drop faint spans, then reuse the existing plain pipeline

`SplitInputRegion` already works on plain text and finds the box borders (drawn with
box-drawing runes in gray, not SGR 2). So the minimal change is a pre-filter:

```
stripAnsiDroppingFaint(colored) -> plain-with-faint-text-removed
DraftOfColored(colored) = SplitInputRegion(stripAnsiDroppingFaint(colored)) → draft, structured
```

`stripAnsiDroppingFaint` is one pass over the string:
- On an SGR sequence (`ESC[…m`): parse the params, update `faint` (`2` → on; `0` or
  `22` → off), emit nothing.
- On any other CSI / OSC escape: emit nothing (as the existing plain capture already
  lacks them).
- On a literal rune: emit it only when `faint` is off.

Result: the ghost span is gone, borders and real input remain → the draft region reads
empty for a ghost-only composer, non-empty for real input. `structured` is preserved
(borders survive).

### D2 — A bounded color capture

`tmux.CaptureFullColor(pane)` = `capture-pane -e -p -S -200` — the color twin of
`CaptureFull`, same scrollback bound. (`CapturePaneColor` exists but uses `-S -2000`
for the mobile history payload; the draft check wants the lighter bound.)

### D3 — Apply to the three "any user draft?" sites, not the payload-matching ones

- `stuckDispatchKind` (`agents.go`): keep the plain `CaptureFull` for `IsStartupGate`
  (the trust-gate prompt is bright, and the gate check is unaffected), and use
  `DraftOfColored(tmux.CaptureFullColor(pane))` for the draft check.
- `wakeDone` guard (`nudge.go`): same swap for its draft check.
- HQ nudge draft-guard (`hqnudge`): inject a color capture (`captureColor`, defaulting
  to `tmux.CaptureFullColor`, overridable in tests) and use `DraftOfColored`.
- `deliver.go` (×4) and `hqnudge`'s wake-ack (`SplitInputRegion` looking for a specific
  batch id): UNCHANGED — they match specific text (the pasted payload / the `#id`),
  which ghost text can never be, so no false positive exists to fix.

## Risks

- **A real draft that happens to contain SGR 2?** Impossible from user typing or a
  paste — only CC's own autosuggestion is faint. Confirmed against the live captures.
- **Faint used elsewhere in the draft region?** The only faint content CC puts in the
  composer is the suggestion; borders/prompt/real input are not faint.
- **Two captures per stuck check** (plain for the gate + colored for the draft): the
  sweep runs over the few TRACKED dispatch panes on the slow tick — negligible, and the
  gate check short-circuits before the second capture.

## Migration

Pure internal detection change. No config, wire, or state-path change. Tests add a
faint-ghost fixture asserting it reads empty while a bright draft still reads non-empty.
