# hq-promote-anywhere — the promotion exit lands wherever the user's rules live

Origin: the commander's 2026-08-17 whole-of-hq review, criterion "flexible enough
for anyone". The measured finding: every teaching surface of the promotion exit —
the brief's hardcoded closing line ("Land it in the gtmux repo (an openspec
change, a seed edit, or code)"), the playbook's charter-level definition, the
docs — assumes the user develops gtmux. A brew-installed user has no gtmux
checkout and has never heard of openspec; for them the exit is a dead loop: they
never promote (three verbs plus a doctor row are dead weight), or HQ promotes by
charter, the brief rots for two weeks, doctor flags it, and the playbook then
escalates to the commander a task the commander cannot perform — manufacturing
exactly the "silent rot" the mechanism exists to end.

## What changes

**Promotion means "this lesson belongs somewhere more durable than this
machine's knowledge base" — the destination is the user's.** The legitimate
landing carriers, taught everywhere the old single carrier was:

- a project's `AGENTS.md` / `CLAUDE.md` (the lesson governs how agents work in
  that repo),
- a team runbook or wiki (it governs how the team works),
- `LOCAL.md` (it governs how THIS supervisor should behave),
- gtmux's own repo — an openspec change or seed edit for developers, **or simply
  a GitHub issue with the brief attached** for everyone else (the brief is
  already a self-contained evidence package).

**The brief closes with the user's target, not with gtmux's repo.** When the
promotion carried `--target`, the closing instruction names IT; without one, a
neutral line lists the carrier options. `land --ref` already accepts any
reference (a PR, an issue URL, "team wiki", a file path) — now the teaching says
so.

**Copy sweeps.** The playbook's charter-level definition and corrections/distill
teaching, the doctor row's flagged note, and `docs/cli.md` all drop the
gtmux-repo assumption to one option among the carriers. `hqPlaybookVersion`
26 → 27.

## Non-goals

No mechanism changes: the ledger lifecycle, the role gate, the doctor floor, and
the escalate-on-stale teaching all stand — a stale brief is still escalation
material, because "pick a carrier and land it" is now something every commander
can actually do. No auto-filing of GitHub issues (gtmux writes into no external
system on its own).

## Impact / risk

Text-only in behavior terms; the one code seam is the brief's closing render
(target-first with a neutral fallback), pinned by tests both ways. Existing
tests that pin the old closing line update with the behavior, as the testing
policy directs.
