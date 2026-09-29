# Support cmux as a host terminal

## Why

cmux exposes `TERM_PROGRAM=ghostty`, so gtmux mistakes its tmux clients and
native agent sessions for Ghostty. A jump then targets the wrong app, and
restore/new cannot open a cmux workspace.

## What changes

- Detect cmux by its workspace environment or `cmux.app` process ancestry
  before the embedded Ghostty identity; register a separate terminal driver.
- Use cmux's AppleScript terminal titles and focus command for precise jumps,
  viewing checks, workspace order, and restore/new workspace creation. This
  also works from the menu bar, where cmux's CLI socket can reject callers
  outside a cmux terminal. Report scripting failures instead of claiming success.
- Include cmux in doctor, appearance, and English/Chinese user documentation.

## Surfaces

| Surface | Change |
| --- | --- |
| CLI / terminal | Detect and control cmux-hosted tmux sessions. |
| menubar | Session jumps and doctor use the cmux driver through the existing CLI. |
| phone / iPad / web | Jumps through the existing server path can target cmux; no UI change. |

## Verification limit

The installed cmux bundle provides the scripting dictionary, but cmux was not
running during development. The driver is covered with AppleScript doubles and
script compilation; live focus, restore, and notification suppression need
an acceptance pass with an open cmux workspace.
