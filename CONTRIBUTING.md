# Contributing to gtmux

Build and gate commands live in `CLAUDE.md`. Run `make check` before every commit;
`scripts/check-design.sh` is the design and architecture half of the same gate.

This file is a maintainer document and stays in English, like
`docs/TROUBLESHOOTING.md` and `docs/release-signing.md`. The bilingual rule covers user
docs; see CLAUDE.md.

## Before you commit a test

Break the thing this test guards. Does it go red? Did you try?

That is the whole ask. What follows is why it is worth the two minutes.

### A check that can only say yes

On the night of 2026-09-09 a sweep through this repo found nine defects. The full gate
was green before it and green after every one of them was found. Seven were not bugs in
the code being guarded. They were guards that did not guard what their name said.

The QR renderer is the clearest. `internal/app/qr.go` carries a footgun note in capitals:
do not shrink the terminal QR with quadrant blocks, it stretches the code and PR #179 was
reverted for it. The test named after that note checked the rendered width against a
budget, under 60 columns. A quadrant render halves the columns. The distortion the note
forbids comes out at 19 columns where 38 are right, passes the budget, and ships. Putting
the quadrant renderer back left all three QR tests green.

The name was right, the comment was right, the intent was right. The assertion pointed
the other way, and nothing in a green run could tell anyone.

### Where the coverage came from

The recurring cause was not carelessness about assertions. It was this: **a guard's
coverage was decided by impression instead of derived from the thing being guarded.**

It shows up in three shapes.

A hand-written list of what to check. `internal/state` grew a guard that stops a test
writing to the operator's real home, with a comment calling that package the one
chokepoint every gtmux path goes through. Twenty-three files resolved `$HOME` on their
own, the hook installer and serve's device roster among them. The claim had never been
checked because nothing could check it.

A hand-written list of which files to open. `internal/docs` compares a documented example
against the real builder so a doc cannot show a format the code will not produce. The
documents it opened were a list with one entry, `docs/cli.md`, while `docs/cli.zh.md`
carries the same marked regions. A fabricated line pasted into the Chinese half passed
the whole gate, and `make docs-fix` left it there. The wake-vocabulary gate in
`check-design.sh` had the same hole in the same two languages.

A threshold picked from memory of the failure. The QR budget above. Also
`mobileapp/e2e`: a default run skipped 19 of 27 suites, exercised 14 of 35 cases, and
exited 0 with a summary that reads like a full pass.

The fix is the same in all three: derive the subjects from the code, and hand-write only
the exceptions. `check-design.sh` finds every wake class by reading the constants, then
requires each one in every file that defines a charter. A deliberate exception goes in a
named allowlist, where it is one line in a diff somebody chose to write.

### Guards that are right today

The hardest version of this passes every honest test you would think to write.

`gtmux doctor` decided whether it can drive a terminal from a hard-coded field beside a
call to the registry that knows the answer. The first test written for the fix compared
the row against `terminal.HasDriver` for the terminals gtmux ships. Putting the
hard-coded copy back passed that test, because the copy was accurate. A copy that agrees
with the registry today reads exactly like asking the registry.

The only way to tell them apart is to present a world the code has not met. The test now
substitutes a registry with a kitty driver in it and requires the row to follow. Against
the copy it fails.

So when your subject is "this reads from X rather than repeating X", ask what would
change if X changed, and put that in the test.

### Say what a guard does not do

When a guard has a boundary, write it next to the guard.

`internal/hq` fingerprints the seeded charter so an edit that forgets to bump
`hqPlaybookVersion` fails instead of shipping to nobody. Nothing there can force the
number up: overwriting the fingerprint for an unreleased version is the right move while
you are still iterating on it. What the guard removes is doing it by accident. That
sentence is in the test, because the next person will otherwise assume it covers more
than it does.

`scripts/check-design.sh` does the same at a larger scale: it states that it checks
claims with a machine-readable source and does not check whether prose is true. A green
run there is not a reviewed document. An honest boundary is worth more than another
assertion that looks strong.
