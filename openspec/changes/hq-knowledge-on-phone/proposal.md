# hq-knowledge-on-phone — the commander can read and close out the knowledge base from the app

## Why

The knowledge base is the supervisor's long-term memory, and on this machine it is not
small: **330 live entries across 7 topics, and 6 promotions pending** — entries HQ has
judged charter-level and written an export brief for, waiting for a human to carry them
somewhere durable. `gtmux doctor` flags a brief that has waited past ~2 weeks, because an
un-carried promotion is exactly the rot the promotion exit exists to end.

Every one of those 6 is waiting on the commander specifically. `promote` is HQ's judgment;
**`land` is not a judgment HQ can make** — only the person who carried the lesson into an
`AGENTS.md`, a runbook, or a PR knows it landed. Today that step exists only as a CLI verb
run from the HQ home on the Mac, so the queue can only be cleared while sitting at the
machine — and the app, which is where the commander actually reads HQ, shows nothing of
the knowledge base at all.

The same gap covers review. HQ writes entries continuously (distill passes, captures folded
in), and nothing outside the Mac ever shows what it recorded. A lesson recorded wrong
compounds: it is echoed into every dispatch, so a bad entry is not inert, it is repeated.
Spot-checking what HQ has learned is a reading task, which is what a phone is good at.

## What changes

- **Two read endpoints** (`GET /api/hq/knowledge`, `GET /api/hq/knowledge/entry`), OWNER
  scope like the rest of `/api/hq/*` — the base carries the supervisor's private
  assessment of the fleet and is not part of a scoped share. The list omits bodies
  (330 entries with bodies is megabytes; without them it is tens of kilobytes) and the
  detail endpoint serves one.
- **One write endpoint** (`POST /api/hq/knowledge/act`) carrying exactly the two verbs a
  phone can honestly perform: `land` (close a promotion with a ref) and `retire` (kill a
  wrong lesson with a reason). Both are one short line of text. `add` and `supersede`
  stay off the phone: they need prose, and prose written on a phone is how a knowledge
  base fills with entries nobody wants to read.
- **The authority rule gets a second, narrower door.** The CLI's cwd-keyed HQ-home gate
  stays exactly as it is — it exists to keep WORKERS out. The serve door is
  owner-authenticated and is the COMMANDER, who outranks the supervisor. A guest is
  refused. Both doors journal the same `gtmux:audit:knowledge` record.
- **A knowledge surface in the app's HQ page**, opened from the header disclosure beside
  the situation board (the two memories, working and long-term, in one place). It leads
  with what is owed — the promotion queue — then the newest entries, then the base by
  topic; an entry opens to its body and provenance and carries the two actions.

## Impact

- Specs: `hq-knowledge` (a second write door + a read API), `remote-access` (three
  endpoints), `mobile-app` (the surface).
- Code: `internal/hq` gains exported domain verbs the CLI and serve both call (the CLI
  verbs become argument parsing over them); `internal/server` gains the handlers;
  `internal/app/serve.go` wires the deps; the mobile app gains a knowledge sheet.
- No change to the ledger format, the render, or the CLI's behavior.
