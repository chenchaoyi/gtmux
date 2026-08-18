# restore-durability — the reboot that showed restore is only as good as what it restores

Origin: the commander's Mac rebooted on the morning of 2026-08-18 with 18 tmux sessions
live. They came back, and then three separate things were wrong at once — a window in the
wrong arrangement, gtmux announcing 18 brand-new sessions it had known for weeks, and a
fabricated "stuck before running — draft" alarm about a project nobody had dispatched
anything to. A read-only audit (`restore-audit-20260818`) traced all three and the
commander approved three fixes.

The three are not three bugs. They are one bug about EVIDENCE, showing up in three
places: gtmux was trusting what it had been told (a configuration, a record, a name)
instead of what it could observe (a file's mtime, a live pane list, a window's shape).

## Why

**1. The autosave was not running, and the check for that asked the wrong question.**

The tmux layout save hangs off tmux's status bar, so it only fires while a terminal is
attached and redrawing. A sleeping Mac saves nothing — however correctly it is
configured. Measured over three and a half days on this machine: **76 gaps, the longest
just under six hours**, against a configured five-minute interval.

gtmux has had a backstop for exactly this since the restore-save-reliability change. It
never fired once, because its gate was `shouldBackstopSave(statusRight)` — "is the
autosave trigger written into status-right?" — and the trigger was written there the
whole time. Written is not running. The reboot restored a save 37 minutes old, and
everything done in those 37 minutes (including the layout change later reported as
"restore broke my layout") was simply not in the file.

**2. Pane-keyed state was never reconciled with the panes that exist.**

A tmux pane id is a per-server sequence number. Restart the server and it reissues from
`%1`, handing old numbers to unrelated panes. gtmux keys a dozen state directories by
that number and cleaned exactly three of them. Measured right after the reboot: 50/52
`enrolled`, 27/29 `goal`, 31/32 `sends`, 93/103 `hqwake` records named panes that had not
existed for up to two weeks — while the live panes now carried those very numbers.

The consequences were not litter, they were cross-wiring. `goal/%11` held what the
commander had told one project's session, and `%11` now belonged to another's. And a
`delivered:false` dispatch record from 2026-08-04, for a session long gone, still named
`%25`; `%25` was now an unrelated project's pane; gtmux screen-scraped it on the strength
of that record, read Claude's resume menu as an unsubmitted draft, and raised the false
alarm. **That misreading of agent chrome as user input was the third in two weeks**
(2026-08-06 dark placeholder text; 2026-08-10 a gate phrase in scrollback).

**3. Nothing ever checked that the restored layout matched the saved one.**

gtmux confirmed session NAMES came back (`waitForRestoredSessions`) and never looked at a
window. tmux-resurrect runs its layout application as `restore_window_properties
>/dev/null 2>&1`, so when `select-layout` refuses a window — "have 3 panes but need 2",
which happens when the restored window ends up with one pane more than the save recorded
— the error is discarded and that window silently keeps the default stacked arrangement.
Two windows did exactly this (2026-08-15, 2026-08-18). Both halves silent means a broken
layout is found days later by the person looking at it, with no evidence left.

## What changes

| | Before | After |
|---|---|---|
| Backstop save | fires only when the autosave trigger is ABSENT from status-right | fires when the save FILE has gone stale — 10 min, or 20 when a trigger is present, so an armed autosaver gets the first move |
| Race protection | the trigger check (which also disabled the backstop) | staleness itself is the interlock (a running autosaver keeps the file fresh) + a yield-and-recheck before the save runs |
| doctor's autosave row | OK whenever one trigger is present | flagged when an armed trigger has not written the save for hours |
| Dead pane records | three turn-state dirs, on the radar's hot path | every pane-keyed family, reconciled against the live pane list at restore and on the serve tick |
| An ancient undelivered dispatch | drives screen-scraping of whatever holds its pane id | keeps its ledger truth, loses its authority over a pane id after two weeks |
| A resume menu on a dispatch pane | falls through to the draft detector → `stuck … draft` | recognized as reopened-session chrome, per-agent through the registry, region- and faint-narrowed like the gate check |
| After a restore | session names counted | every saved window's pane count + arrangement compared against the live one; drift named on the terminal and in `restore.log` |
| Save age at restore | warned only past 24 h | always printed: "Restoring the layout saved at 09:57 (37m ago)" |

## What this deliberately does NOT do

- **It does not stop the extra pane appearing.** The cause of a window coming back with
  one pane more than the save recorded is still open (the audit could not reproduce it —
  the two instances were gone by the time it looked). This change ends the SILENCE so the
  next occurrence is caught in the act, which is what the audit asked for.
- **It does not delete stale dispatch records.** `gtmux reap` needs them, and the ledger
  saying "this goal never landed" stays true forever. Only the record's authority over a
  reused pane id expires.
- **It does not enrol restored panes into the known-panes baseline** (audit problem 4 —
  the 18 "new session" wakes). Not in the approved scope; the sweep neither helps nor
  hurts it.
- **It does not touch `resume/` or `usage/`.** Those are keyed by locator and
  conversation id, and `resume/` must survive the one moment every pane is gone.

## Impact / risk

The risky direction is the backstop: the old gate existed because two concurrent
`save_all` runs over the same files once produced paired save files and a truncated
`pane_contents.tar.gz` — gtmux corrupting the save restore depends on. That lesson is
kept, expressed as evidence instead of configuration: an autosaver that IS running keeps
the file fresh, so the backstop never wakes beside it; an armed one gets a wider grace;
and the one moment both plausibly fire at once (a wake from sleep, when the status bar
redraws and serve's tick resumes together) is covered by yielding for a few seconds and
re-checking before the save actually runs.

The second risk is the sweep deleting something it shouldn't. It is bounded three ways:
only files whose NAME is a pane id, only the enumerated families, and never on an empty
live set (an unreadable `list-panes` must read as "unknown", never "no panes exist").
