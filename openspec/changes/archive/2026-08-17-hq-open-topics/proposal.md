# hq-open-topics — the topic vocabulary opens through the ledger

Origin: the commander's 2026-08-17 whole-of-hq review, criterion "anyone who uses
it learns personalized knowledge from their own usage". The measured finding: the
seeded KB README says **"Add topic files as needed"** (hq.go:497) while
`gtmux knowledge add --topic clients` refuses with `unknown topic` — a promise the
code rejects on first contact. A data engineer needs `datasets`; a freelancer
needs `clients`; today both hit a closed six-word vocabulary that no spec
requirement even declares closed (the constraint exists only as implementation
behavior).

## What changes

**A topic becomes a ledger operation.** `gtmux knowledge topic <name> --desc "…"`
appends a `topic` operation — named, described, journaled, with provenance like
every other mutation — and the topic vocabulary becomes: the six built-ins plus
every ledger-declared topic. Everything downstream reads the same source:

- `gtmux knowledge add --topic <custom>` accepts it; `gtmux capture "… @<custom>"`
  accepts it too (workers can file candidates under the user's own domains).
- The topic renders immediately (marker + name + the description as its intro),
  so declaring a topic is visible, not a silent registry write.
- The dispatch-time knowledge echo consults every CUSTOM topic alongside the
  built-in pitfalls/workflows — a user's own domain topics are exactly the
  lessons they want surfaced at dispatch; the built-in exclusions
  (accounts/corrections/environment: deliberately not dispatch-time context)
  stand unchanged.

**The 5-vs-6 inconsistency closes.** `gtmux capture` accepted five topics while
`knowledge` accepted six (`environment` was excluded from capture with no
documented reason a user could find). Capture now accepts every valid knowledge
topic — built-ins, `environment`, and custom — with one shared validation.

**Names are bounded and honest.** A topic name is a slug (`[a-z0-9-]`, ≤ 40
bytes); collisions with built-ins, existing customs, and the reserved names the
directory layout owns (`README`, `legacy`, `promotions`) refuse loudly.

**The false promise is corrected at its source.** The seeded README's "Add topic
files as needed" becomes "Add topics with `gtmux knowledge topic <name> --desc …`";
the playbook's KB section teaches the verb (`hqPlaybookVersion` 25 → 26); the
docs state the vocabulary rule instead of leaving it to be discovered by error.

## Non-goals

- **Topic removal.** Entries drive everything; an unused declared topic costs one
  ledger line and an empty render. A removal lifecycle is deferred until someone
  actually needs it (Known Limitations).
- **Per-topic echo configuration** — custom topics echo, built-in exclusions
  stand; a tuning surface waits for measured need.
- **Migrating any hand-made topic file** a user created against the old advice —
  the legacy mechanism from `hq-knowledge-ledger` already covers it on first
  ledger touch.

## Impact / risk

The risky direction is a validation seam that drifts: capture and knowledge
judging topics differently again. Both now call one function over one source,
and the tests pin a custom topic through the WHOLE loop (declare → capture under
it from a worker cwd → accept the candidate into an entry → see it echoed at
dispatch). Playbook v26.
