# chat-time-separator — a mark where time actually passed

## Why

The chat labels a turn whenever its formatted time differs from the turn above it. The
label carries HH:MM, so "differs" means "a minute later", and a normal back-and-forth gets
a timestamp above almost every turn. A mark that appears on nearly every turn marks
nothing: the reader scrolling back for where they left off has to read the clocks rather
than see the break.

The commander asked for the break to be drawn as a break 「在chat模式下，如果时间间隔超过一定
时间等情况，参考一下这个样式分隔」, pointing at a chat that draws it as a centred time flanked
by a wavy rule.

## What changes

**It marks a break, not a minute.** A separator appears where the conversation actually
stopped and started again:

- the first turn that carries a clock, so the history has a beginning;
- a calendar day change;
- a gap of 15 minutes or more from the turn before it.

Everything else carries nothing. Fifteen minutes is long enough that a working
back-and-forth shows none and short enough that a real pause shows; it is one constant.

**It looks like a break.** The label sits centred between two wavy rules, the way the chat
apps the commander reads draw it, rather than floating as a bare line of grey text. The
wording is unchanged: 今天 23:46 · Yesterday 09:12 · 9月19日 10:40, already localized.

The session seam (a `/clear` or `/new`) stays what it is and keeps its own line: it says a
DIFFERENT conversation starts here, which is not the same statement as "time passed".

## Surfaces

- **终端 (terminal)** — not applicable: the CLI has no conversation view.
- **菜单栏 (menubar)** — not applicable: the HQ card reads the board and the knowledge
  base, never a transcript.
- **手机 (phone)** — done: the separator replaces the per-turn label in the chat view.
- **iPad** — done by the same implementation, which the phone and iPad share.
- **Web (web)** — done: the browser mirror's chat draws the same separator, with the wave
  as a repeating background rather than a measured SVG.

## Risk

A reader who used the old label to date an individual turn loses that: between separators,
a turn no longer carries its own clock. That is the point of the change, and the turn's
time is still one tap away in the terminal view and in the session log. If it turns out to
be wanted, the answer is a per-turn time on demand, not a label above every turn.
