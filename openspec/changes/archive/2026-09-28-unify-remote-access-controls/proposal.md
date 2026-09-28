# Unify remote access controls in the menu bar app

## Why

Preferences and Pair your phone change the same Mac setting, but render it as two
different interfaces. Preferences uses labeled rows inside Remote access; the pairing
window shows a wide access picker and a narrower route picker in separate sections.
The difference makes one setting look like two unrelated choices.

## What changes

Both windows use one shared control for Access (Off / Local network / Anywhere)
and, when Anywhere is on, Route (Standard / Direct). Both show the same labels,
order, help, and segmented control treatment. The pairing window keeps Direct
server measurements below Route and the pairing code below the access card.
Existing confirmation, Pro unlock, Direct code, server move, and URL refresh
behavior stays attached to each window.

## Surfaces

- **终端 / terminal**: no change; the CLI remains the state source.
- **菜单栏 / menubar**: Preferences and Pair your phone share the access controls.
- **手机 / phone**: no change; it reads the resulting address as before.
- **iPad**: no change; same mobile behavior as phone.
- **Web**: no change; the guest surface cannot change this setting.
