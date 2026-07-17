# Design: docs-drift-guard

## Context

Two gates exist today and neither covers this:

- `check-design.sh` — grep-shaped invariant checks (`note` + `fail=1`), including §6's
  command-registry completeness. Its own comment scopes itself out of prose truth.
- `openspec validate --specs --strict` — proves specs are well-FORMED, never that they
  are TRUE.

So the machinery to hang this on already exists; what's missing is the checks.

## Decisions

### 1 · Test what has a source of truth; refuse to fake the rest

The line between "checkable" and "not" is whether the claim has a machine-readable
source:

| claim in a doc | source | checkable |
|---|---|---|
| `» gtmux·done  … │ …` (a format) | `hqwake.Line` | **yes** — render and compare |
| the class table's members | `hqwake.Class*` constants | **yes** — set comparison |
| `[gtmux] …` (a retired format) | the change that retired it | **yes** — denylist |
| "`done` fires for ANY session, not just a dispatch" | `wakeDone`'s control flow | **no** |

The fourth row is the honest limit, and it matters that this design states it: #478's
wrong `done` trigger — arguably its worst error, since it misdescribes *when the system
speaks to you* — would have survived Phase A. A guard that implies otherwise is worse than
no guard, because it retires the vigilance that was catching things.

### 2 · The example registry lives with the code, not the doc

```go
// docExamples maps a doc region id → the line the code actually produces for it.
var docExamples = map[string]func() string{
	"wake-waiting": func() string {
		return hqwake.Line(hqwake.ClassWaiting+"·permission", "api:0.0 (%7)", `title:"run the tests?"`)
	},
	"wake-done": func() string {
		return hqwake.Line(hqwake.ClassDone, "web:2.0 (%11)", "3m",
			`goal:"fix the login bug"`, `tail:"tests pass"`) + " · #a3f1c2"
	},
}
```

The registry is Go, so it cannot compile against a builder that changed shape — which is
one more link in the chain the doc gets to borrow. The doc side is inert: a single
`<!-- gtmux:rendered <id> -->` immediately above a fenced block, invisible when rendered.

**Changed during implementation:** this design first called for PAIRED markers wrapping
the fence. The fence already delimits the block, so a closing marker is a second
delimiter that can disagree with the first — one more failure mode for zero gain. The
marker opens; the fence closes. A marker NOT followed by a fence is an error, not a skip:
it means someone meant to check an example and silently wouldn't have.

**Why a registry rather than parsing the doc's fence and guessing what produced it?** A
line like `» gtmux·done  …` is ambiguous about which call made it (class? head? which
fields?). Naming the region ties it to one call explicitly, and the id is the thing a
reviewer greps when they change the builder.

**The failure mode we accept:** a doc example nobody registered is unchecked. That's the
same shape as §6's `HIDDEN` — an escape hatch you must take deliberately.

### 3 · Enumerations: compare sets, name the surfaces

The wake vocabulary is taught in exactly two places, for two audiences: `docs/cli.md`'s
table (the human) and `hqInstructions` (HQ itself). Both must name every class, or someone
is operating on a partial map — which is what #478 did to every reader for a version, and
what a missing class in the playbook would do to HQ *right now* (it would receive a knock
whose class its charter never mentions).

The check is a set difference against `grep -oE 'Class[A-Za-z]+ *= *"[^"]+"'` over
`internal/hqwake/wake.go`, with a `DOC_HIDDEN` allowlist. It lives in `check-design.sh`
beside §6 rather than in Go: it is a repo-hygiene rule about files, and its neighbors are
already there.

### 4 · The denylist is an audit trail, not a pile of greps

Each entry carries the change that retired the token:

```sh
# token                        # retired by
'[gtmux] '                     # hq-perception-v2 — the wake format is now '» gtmux·<class>'
'internal/menubar/'            # the Swift migration (v0.0.11) — that package is gone
'important` = the attention'   # hq-attention-stream — `important` is the ESCALATION subset
```

Without the second column this becomes a list nobody dares delete from. With it, an entry
is removable the day its change is forgotten — and the list doubles as the answer to "why
does the doc not say X anymore".

### 5 · `make docs-fix` writes; CI only reads

The test fails with a diff and does not mutate; a separate `-update`-style entry point
(`make docs-fix`) rewrites the regions. CI never rewrites files, and a contributor never
hand-copies a rendered line — the loop is: change the builder → CI goes red → `make
docs-fix` → commit.

## Risks / Trade-offs

- **False confidence** — the headline risk, addressed in §1 by stating the boundary in the
  spec itself rather than in a comment nobody re-reads.
- **A false RED on an illustrative example** (a doc line that means to be approximate).
  Mitigated by markers being opt-in: an unmarked fence is untouched.
- **Marker noise in the markdown source.** Two HTML comments per example, invisible when
  rendered. Cheaper than the audits they replace.
- **The registry can rot** — a region deleted from the doc leaves a dead id. The test
  should fail on a registered id with no region, not only the reverse.

## Migration

Nothing to migrate: the checks pass on today's tree (#478 already made the two wake
examples byte-accurate and completed the class table — by hand, which is precisely the
labor this change automates). The first Phase-A run should therefore be green, and if it
isn't, it has already found something.
