# Design: hq-attention-stream

## Context

Three things must agree and currently don't:

1. `events.Severity(r Record)` — a pure, LLM-free classifier over one record.
2. The seeded playbook — what HQ believes it should read.
3. The spec — which currently REQUIRES both of the above to be what they are, so this
   isn't drift to correct silently; it's a decision to reverse.

## Decisions

### 1 · The verdict rides the record, because Severity is pure

`Severity` takes a `Record` and nothing else — no HOME, no tmux, no transcript. That
purity is load-bearing (it is why severity can be stamped once at the single append path
and read back forever without recompute). So "is this submission a real instruction?"
cannot be answered inside `Severity`; it must arrive ON the record:

```go
// Additive (hq-attention-stream): who/what a prompt submission carries.
Origin string `json:"origin,omitempty"`

const OriginInstruction = "instruction"
```

```go
case "UserPromptSubmit":
	if r.Origin == OriginInstruction {
		return SevNotable // an instruction reached a session — a fleet change
	}
	return SevRoutine // harness injection, an echoed wake line: not an act
```

Absent field → routine → **exactly today's behavior for every legacy record**, which is
what makes this additive rather than a rewrite of history.

### 2 · One classifier, two consumers

The hook already computes precisely this verdict for the `goal-changed` wake:
`goalOf(payload.Prompt)` returns the instruction (prose, or `(slash-command) /compact`)
and `""` for content the user did not author. So `origin` is stamped from the same call —
no second classifier, no chance of the wake and the tier disagreeing about what a user
act is.

That the slash case works at all is #474's doing: before it, `CleanUserPrompt` rejected
`<command-name>/compact</command-name>` and the whole submission read as nothing.

### 3 · Author-agnostic, on purpose

`origin: instruction` says "this submission carries a real instruction", NOT "a human
typed it". The alternative — proving a human typed it — needs to rule out gtmux's own
dispatch, and we can't do that reliably:

- `dispatch.RecentSend(pane)` stores a **payload hash**; the harness appends
  `<system-reminder>` blocks to the prompt it reports, so the hash misses on exactly the
  prompts we'd need it to hit. (Land-verification faces the same problem and solves it
  with `NormalizeHead` matching — but the send side never records a head, only the hash.)
- Recording a head too would work, and is the obvious follow-up if someone later needs
  the distinction — but it changes `dispatch.IO.RecordSend`'s signature, and this change
  doesn't need it.
- A pure **timing** heuristic ("gtmux typed here 10s ago → machine traffic") fails in the
  dangerous direction: a user typing right after a dispatch is silently reclassified as
  machine traffic, which is this bug all over again.

Asymmetry decides it: a dispatch echo landing in `notable` costs a line in a stream HQ
already reconciles against its ledger; a human instruction landing in `routine` costs the
instruction. When the classifier is unsure, it must guess "instruction".

### 4 · Three streams, named — the wording is the primary fix

The severity bump alone doesn't close the hole: an HQ pulling `--severity important` still
misses `notable`. What actually closes it is HQ knowing what each read IS.

- `--since-seq <n>` — the delta. **The reconcile path.** Already what the wake protocol
  tells HQ to run on a wake; the playbook just contradicted itself two sentences later.
- `--severity notable` — the fleet-change stream: instructions, turn-ends, lifecycle.
- `--severity important` — the escalation stream: blocked / asking / crashed. A SUBSET.

Plus the rule that generalizes past this bug: **a filtered read is a triage shortcut, not
your model of the world.** Any filter is a claim about what doesn't matter, and this whole
change exists because such a claim was wrong once.

### 5 · Why not raise it to `important`?

Because then `important` means nothing. It is the tier HQ triages FIRST and (per the
attention system) the one that survives `gtmux quiet`. Filling it with every prompt
submission — including every dispatch echo — buries the pane that is actually blocked.
`notable` is the tier whose existing members are exactly analogous: a `report` turn-end,
a `SessionStart` — things that change the picture without anyone waiting on HQ.

## Risks / Trade-offs

- **The notable stream gets busier** — every dispatch echo joins it. Accepted per §3; it
  is also arguably correct (a dispatch landing is a fleet event).
- **Playbook v4 rewrites live homes' AGENTS.md** (backing up v3). That is the versioned
  playbook working as designed; personalization lives in the untouched LOCAL.md.
- **A v3 brain against v4 code** reads the same records with an older map: it would pull
  `--severity important` and still miss instructions until `gtmux hq` re-seeds. The code
  change is strictly additive, so nothing breaks — the blind spot just persists until the
  upgrade, exactly as it does today.
- **`hq_test.go` pins `--severity important` in the playbook text.** The string survives
  (the escalation stream still exists), but the assertion's intent — "read the filtered
  stream, not raw lines" — is now wrong and the test's comment must change with it.
