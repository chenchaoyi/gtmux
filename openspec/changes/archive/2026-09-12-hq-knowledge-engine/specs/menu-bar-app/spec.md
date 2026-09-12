# menu-bar-app (delta)

## ADDED Requirements

### Requirement: The HQ window shows an entry's three axes and carries it

The knowledge tab SHALL show kind, provenance (with count) and audience on an entry as
the four audience words (中控 · 本机 · 仓库 · 全体 / hq · machine · repo · everyone), and
SHALL offer "write it in" for `hq` / `machine` / `repo` (preview, append, land) and
"feedback to gtmux" for `everyone`, plus `withdraw`. The pool view SHALL group by
neighbourhood.

#### Scenario: Carrying a machine-wide lesson

- **WHEN** the user confirms "write it in" on a `machine` entry
- **THEN** the canonical file and every agent block are refreshed, the entry lands with
  the canonical path as ref, and doctor's sync row is green
