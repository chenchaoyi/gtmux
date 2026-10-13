# Menu-bar upgrade notes

## Why
The phone shows what changed after an upgrade; the menu-bar installer relaunches
without an equivalent reading surface. CLI output can be hidden during a detached
install, leaving users unable to discover or revisit the changes.

## What Changes
Provide a once-per-upgrade reading window and permanent menu/preferences entries.
Read bilingual stable-release notes through the existing core, bounded to the
installed app version; retain skipped-version grouping and language fallback.
Cache notes locally, acknowledge only successful reads, retry failures and disclose
the latest-30-release source boundary. Migrate the existing app-launch marker so
current users can see the first upgrade that adds this feature.

## Surfaces
- terminal: additive `whatsnew --json --until` protocol; existing text defaults remain.
- menubar: upgrade window, cache/acknowledgement, permanent entries and retry.
- phone: existing What's new archive/window unchanged; shared interaction semantics.
- iPad: same existing mobile archive/window; no desktop updater.
- Web: not applicable; the browser does not install a menu-bar binary.
