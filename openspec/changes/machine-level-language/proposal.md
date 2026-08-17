# machine-level-language — one language for every gtmux process

Origin: the commander's fourth whole-of-hq review (2026-08-17), the round's
one finding.

## Why

Processes do not share an environment. A launchd-started `gtmux serve` and a
hook subprocess have no `GTMUX_LANG` and none of the user's shell locale — so
the wake suffixes and desktop notifications they emit resolve their language
independently of what the user's own terminal shows. The locale fallback
(empty-gap-and-locale) widened the zh path and with it the odds of the
mismatch: a Chinese shell beside an English-speaking serve, on one machine.

## What changes

- `gtmux config lang [en|zh|auto]`: the machine-level language, stored in
  config.json where every process reads it. `auto` explicitly means "follow
  the system locale". No value shows the resolved language.
- The resolution order everywhere becomes: `--lang` > `GTMUX_LANG` >
  `config.json lang` > system locale > `en`. A broken value at an explicit
  layer resolves to English rather than silently falling through (except
  `auto`, whose whole meaning is the fall-through).
- docs/cli.md: the language line and a `### lang` section under `gtmux
  config`.

## Non-goals

No per-surface languages (the board/LOCAL.md seed-time rules stand); no
migration — an absent key behaves exactly as before this change.

## Impact / risk

One resolver, table-tested (11 cases); the new key is additive and optional.
