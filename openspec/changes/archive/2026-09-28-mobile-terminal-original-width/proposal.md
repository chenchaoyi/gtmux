# Preserve wide terminal rows on the phone

## Why

The iPhone terminal always rewraps a captured Mac pane at phone width. A 189-column
Codex pane becomes roughly four phone rows per Mac row; full-line diff colors and
prompt boundaries break apart. The existing design deferred horizontal viewing
after a nested iOS scroll view rendered blank.

## What changes

The pane snapshot includes the Mac pane's column count. The phone keeps its
readable Wrap mode and adds an Original width mode that preserves captured row
boundaries and pans horizontally. The horizontal viewport encloses the vertical
terminal scroller; the iOS color grid and native selection layer retain the same
width and row geometry. Older servers without a column field fall back to the
widest captured row. A small terminal control switches the modes without changing
the Mac pane size.

## Surfaces

- **终端 / terminal**: no CLI command changes; tmux supplies the pane width.
- **菜单栏 / menubar**: no UI change.
- **手机 / phone**: Wrap / Original width toggle in the terminal viewer.
- **iPad**: same viewer and choice, with a wider viewport.
- **Web**: no change; its xterm renderer already has its own layout.
