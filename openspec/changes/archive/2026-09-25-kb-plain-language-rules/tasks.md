# Tasks

## The table
- [x] `internal/knowledge/plainlang.go`: the rule record (number, tier, mechanical or
      judgement, one-line instruction, why, before/after pair), en + zh halves
- [x] fill the table: the mechanical rules the lint already runs, the four new mechanical
      ones, and the judgement rules the charter paragraph carries today
- [x] the two rules the humanizer skill has no reason to carry: everything executable stays
      verbatim, and an entry says who / where / what happened / what to do
- [x] test: every rule has both language halves, a before that its own matcher finds, and
      an after that it does not

## The lint reads the table
- [x] `voice.go` takes its matchers from the table instead of its own list
- [x] new checks: a guess presented as a fact; a title restated as the body's first
      sentence; an implementation name standing as a title
- [x] the fourth candidate, the id's slug in the title, measured at 457 of 720 entries and
      traced to the index never printing the id — fixed in the render, kept in the table as
      a judgement
- [x] each new check confirmed by re-introducing the defect and watching that check fail
- [x] measure the new findings against the real base before shipping: 720 entries, 12 for
      the title echo, 24 for the identifiers, 0 for the guess, and the base is at 10%
      ai-voice / 3% title-unreadable

## The command
- [x] `gtmux knowledge style` prints the table; `--json` gives it structured
- [x] `docs/cli.md` + `docs/cli.zh.md`: the `knowledge` section gains `style`
- [x] help entry so `gtmux knowledge --help` lists it

## The playbook
- [x] `hq.go` + `playbook_zh.go`: the `kb-plain-language` paragraph shrinks to the judgement
      half and points at `gtmux knowledge style`
- [x] bump `hqPlaybookVersion` (a change to `hqInstructions` that skips this reaches no
      existing home)

## The summary line
- [x] `machineIndex`: the title on its own line, the summary and the id under it, no ` — `
      join, and a leading copy of the entry's own slug dropped from the title
- [x] tests that fail if the dash comes back, if the id stops being printed, or if the
      render stops dropping the slug (the render goldens carry no machine-audience entry,
      so none needed updating)

## Close
- [x] `make check` and `scripts/check-design.sh` green, judged by exit code
- [x] sync-specs + archive in the same PR or the next
