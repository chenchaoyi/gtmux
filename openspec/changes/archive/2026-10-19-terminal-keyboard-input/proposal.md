# Shared terminal-key input

## Why

Codex can show a folded question whose entry requires Shift+Left. The phone cannot
send that chord, and F2 or another new shortcut would need another hardcoded client
button and server allowlist entry. Users need a general terminal-key input surface
that stays understandable as agent keyboard behavior evolves.

## What Changes

- Introduce a shared catalog and encoder for navigation/editing keys, F1–F12,
  shortcut characters and explicitly supported Ctrl/Alt/Shift combinations.
- Add a visible Keys panel entry beside the phone's keyboard toggle; use one-shot
  modifiers and Navigation, Function and Characters categories. Adapt it to iPad
  and provide equivalent on-screen keys in web terminal mode.
- Advertise pane-scoped capabilities and extend `/api/send` with typed key events,
  retaining valid legacy requests and server-side input grants.
- Validate before terminal side effects, bind new sends to the current server/pane,
  and provide bounded deduplication for new key/action events, including duplicates
  in flight. Keep uncertain outcomes explicit across disconnects/restarts.
- Let agent adapters supply contextual actions through the same key-input path.
  The Codex question action requires verified live context and keybinding evidence;
  unsupported event sources do not block the generic keyboard.

## Impact

Proposed capability deltas: remote-access and mobile-app. The shared encoder is a
cgo-free leaf; all clients consume its catalog rather than implement their own
terminal encoding. Current capability specs and production API documentation are
unchanged because no feature implementation is included in this design delivery.

Research and decisions: [English](../../../../docs/design/terminal-keyboard-research.md)
and [中文](../../../../docs/design/terminal-keyboard-research.zh.md).

## Surfaces

- terminal / CLI: reuse the key catalog for named input; raw attach remains separate.
- menubar: retain pane focus; do not add a second full terminal keyboard.
- phone: visible Keys entry, adaptive sheet and one-shot modifiers.
- iPad: the same data/rules with an anchored panel or compact sheet.
- web: equivalent on-screen keys and grants in terminal mode; browser shortcuts stay local.

## Non-goals

The initial research proposal was subsequently approved for implementation in this
change. Acceptance limits are recorded in tasks.md. No production release, real
user/agent pane input, runtime installation or access-policy change is included.
No full terminal emulator, arbitrary byte/command injection, macros, modifier lock,
auto-repeat, offline replay, F13–F24/media keys, global hardware-key capture,
tmux configuration changes, new status class or native question-answer lifecycle.
No promise that every OS shortcut or custom agent keybinding can be sent or inferred.
