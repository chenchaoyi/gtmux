# Fix terminal screen history boundaries

## Why
The user marked a jump from old git output to the latest Codex turn. On live pane %19, tmux reports alternate_on=1 and unchanged history_size=34738. A capture with -S -2000 joins 2000 rows from the frozen normal-screen history to 48 current alternate-screen rows. Increasing the capture/render limits cannot restore the missing intermediate TUI history.

## Changes
- Choose current-screen-only capture for alternate-screen applications, and the existing bounded scrollback for normal screens, inside one tmux command queue. Do not clear histories or toggle running applications.
- Direct interactive Codex commands created by gtmux (spawn, HQ, standard restore/adopt) default to --no-alt-screen. Preserve explicit alternate-screen choices and custom launchers. Keep the resume identity parser/registry unchanged.
- Add isolated real-terminal regression tests for normal, alternate and returning-to-normal transitions, ANSI/trailing rows and stale acknowledgement text. Test all launch entry points and launcher/option preservation.

## Boundaries
Existing running clients are not restarted; a later restart with the inline flag enables future terminal history, not retroactive recovery. Chat retains the persistent conversation. The 2000/1000 row limits are separate and unchanged. No global Codex settings, credential, notification, binding or ownership changes.

## Surfaces
- CLI / terminal: bounded read-only capture and managed direct Codex launch/resume commands corrected.
- Menu bar (menubar): capture consumers share the corrected core; no new UI.
- Web: the existing pane API supplies a truthful current frame rather than unrelated normal history.
- iPhone / phone: same corrected API, no client package required for the capture fix.
- iPad: shared API rule; no device-layout acceptance claimed.
