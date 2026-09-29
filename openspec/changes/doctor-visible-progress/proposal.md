# Doctor progress and bounded Homebrew probe

## Why

`gtmux doctor` evaluated all sections before writing its first line. A blocked
probe therefore looked like a command that had not started. The optional `brew
outdated` lookup in the tmux row had no deadline and could wait indefinitely.

## What changes

- Announce each check section on stderr before evaluating it in an interactive
  terminal. `--progress` opts into the same trace for redirected output.
- Disable Homebrew automatic updates for the optional version lookup and bound
  the wait. An unavailable update suggestion must not block the health report.
- Keep the report, tally, exit status, and consent flow intact.

## Surfaces

| Surface | Change |
| --- | --- |
| CLI | Doctor prints stage progress to stderr in a terminal; `--progress` enables it when piped. |
| menubar app, phone app, iPad app, web, terminal jump | None. |
| Remote access and production services | None. |

## Scope

CLI doctor and its help/docs only. No service, app, or production setting changes.
