# Change: board-ask-reply

> STATUS: implemented 2026-09-14 (same PR). On the commander's direction, pointing at the phone's situation board: 「situation board 里有很多我需要处理的信息，但是没有直接处理告知 hq 的入口，这里能否增加方便的针对选定事项告知 hq 如何处理的入口」.

## Why

The board's commander section (「还等你定的」, lifted to the top on 2026-09-09) is a numbered
list of decisions only he can make, and HQ writes its recommendation into most of them. The
sheet was read-only: to answer item 3 he closed the sheet, found the composer, and typed
"about item 3 on the board…" from memory. The distance between reading a decision and
giving it is exactly what a chief of staff is supposed to remove.

## What Changes

1. **Each item of the commander's section is a row** (parsed from HQ's own numbering and
   bold group headings — `boardSections.askItems`), with the group kept as a label and a
   "Tell HQ ›" at the right.
2. **Tapping an item asks how to answer it**, the two things a commander says to a chief
   of staff: **"Do as you suggest"** (offered only when the item carries a recommendation)
   sends `态势板「还等你定的」第 3 条（…）：按你的建议办。` to HQ at once; **"Let me say…"**
   closes the sheet and puts that quote in the composer with the cursor after it, so he
   writes the decision and sends. The quote names the item by HQ's number and first line,
   so HQ knows which one is decided without re-reading.
3. The composer gains a `prefill` input (text + nonce) for exactly this hand-off, and the
   HQ page's long-declared `prefill` prop (the radar's pane mention) now reaches it too.

## Surfaces

- 终端 (terminal, incl. remote attach): not applicable — the board there is `gtmux hq --board`, a print.
- 菜单栏 (menubar): unchanged by rule — the Mac reader reads and judges, it does not drive the fleet (DESIGN §12); the reply is a send, which lives on the phone/web.
- 手机 (phone): the subject of this change.
- iPad: the same sheet, the same rows; the composer is permanent in the regular shell.
- Web: not applicable — the shared page carries no board.

## What does NOT change

- The board's text, HQ's numbering, the read-only nature of everything else on the sheet.
- No new API: the reply is the existing send to HQ's pane.
