# row-detail-sheet — long-press says something the row cannot

## Why

Long-pressing a session on the phone opens a system alert containing the agent's name
and the task — **both already on the row it was pressed from**. One Close button. The
gesture costs a deliberate 350ms hold and returns nothing that was not already visible.

It exists for a real reason: the row clamps the task to one line, so a long task (or an
error summary) is otherwise unreachable. That reason is worth keeping. Repeating the row
back is not.

Two things a reader actually needs there, and cannot get anywhere else on this screen:

- **The full truth of this row** — the whole task, the whole error, how long it has been
  in this state, and WHICH pane it is. A truncated error (`You've hit your weekly limit ·
  resets Aug 28 at 1…`) is exactly the text you long-press to read.
- **The one step you would otherwise hunt for** — jumping the Mac's terminal to this
  pane. Today that lives only in the pane browser, two screens away, though the radar is
  where you notice something needs a look.

## What changes

A bottom sheet, not a system alert. An alert cannot carry sections, an icon, or more
than a line or two, and half of what reads as "threadbare" is that it is a system dialog
being asked to do a job it has no shape for.

Four blocks, each present only when it has something to say:

1. **Identity, always first.** `%60 · session:window.pane`, the window's stable id and
   name, project and branch. The pane id leads, for the reason the share-picker already
   settled: names are a gloss and ids are the anchor — two panes running one project
   look identical once a row truncates them.
2. **What it is doing.** The full task, unclamped. The full error text (amber ⚠) or the
   background-work note (⧗). Status and duration in the words the status language
   already uses.
3. **What you can do from here.** Jump the Mac's terminal to this pane; copy
   `gtmux focus %60` (the same token, the same shape, on all three surfaces); Diff, when
   the pane has a repo; and Open, which is what a tap does.
4. **Nothing else.** No destructive action — long-press is a browsing gesture, and reap
   or kill behind it is a misfire waiting to happen. Nothing the detail screen already
   offers (composer, quick replies, font size): that is one tap away.

## Every kind of row says its own truth

The sheet is opened from a list that holds four different kinds of thing, and today all
four produce the same two lines. That is not terse — for two of them it is wrong.

| row | identity | actions | what must differ |
|---|---|---|---|
| agent in tmux | full | all | the baseline |
| native (not in tmux) | no `%N`; says so | jump and send disabled, **with the reason** | gtmux senses these read-only; offering a jump that cannot work is a lie |
| watched plain pane | `%N` + its command | jump, copy | it has NO agent status — rendering "idle" for it invents one |
| errored session | full | all | the error text is the most important thing on the row and is the thing that gets truncated |

## Boundaries

- The jump has a side effect on the user's Mac (it moves the terminal). It is included
  deliberately — noticing something on the phone and going to look at it on the Mac is
  the natural next step — but it is a labelled button, never the sheet's default action.
- This replaces an alert whose only job was revealing clamped text. The row itself does
  not change.
