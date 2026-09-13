# Knowledge-engineering research: what the supervisor's knowledge base should learn, and from whom (2026-09-12)

> Background: the supervisor's knowledge system is moving from "recording rules" to "continuous learning" (mining → candidate pool → distill → ledger →
> distribution by audience → recurrence feedback). Before touching the classification and distribution mechanics, survey how the field organizes
> knowledge today, then judge what is worth borrowing and what is explicitly not. This document only compares and judges; it decides no implementation.
> The current three-layer ownership is in `knowledge-layers.md`; the mining mechanism is in `openspec/changes/archive/2026-09-11-hq-transcript-mining/`.

## Conclusion first

Nine approaches were examined. In **organizational form** they converge strongly: atomic entries, entries connected by links rather than a directory tree,
one immutable body of raw material, compilation rather than retrieval, and a lint that runs periodic health checks. The supervisor's ledger already has four of these
(atomic entries, `[[family]]` links, provenance with a seq, deterministic rendering); what it lacks is lint, pre-grouping of similar entries,
and progressive disclosure at distribution. On classification frameworks none can be lifted as is: PARA and Johnny Decimal are for people managing projects
and files, not for agents accumulating experience; what does apply is CoALA's three-way split (what happened / how the world is /
how things are done) plus a "who needs to know" audience axis.

In one sentence: **in form, learn from the Zettelkasten lineage; in classification, use CoALA; in process, use Karpathy's
"compile + lint"; in distribution, use Skills' progressive disclosure; PARA / graph databases / vector retrieval are not introduced.**

## Nine approaches, one paragraph each

### 1. Karpathy's LLM Wiki (2026)

Raw material goes in `raw/` and is never modified; the LLM compiles it into an interlinked markdown wiki in `wiki/`;
questions are then answered from the wiki, not the raw material. Three operations: ingest (new material in, only the affected pages change),
query (answer with citations), lint (check index completeness, broken links, health). The core claim is moving the work from query time
to compile time: RAG rebuilds the relationships every time, the wiki writes them down once and keeps repairing them.
([concept](https://denser.ai/blog/llm-wiki-karpathy-knowledge-base/) ·
[one implementation](https://github.com/Astro-Han/karpathy-llm-wiki))

Comparison: the supervisor's ledger **is already compiled**: entries are the product of distillation, topic files are a deterministic render of the ledger,
and `gtmux knowledge` queries the ledger, not the event stream. Two gaps. One is "raw is immutable and every page cites back to raw":
our provenance is an event seq and a session id, and a mining candidate carries a conversation excerpt, but once in the ledger **the excerpt is not kept**; the entry
retains only the pointer. The other is **lint**: we have self-check auditing the supervisor's output, but no health check dedicated to the ledger
(orphan entries, broken `[[links]]`, synonymous entries, stale entries never superseded).

### 2. The Obsidian ecosystem and obsidian-mind

Obsidian itself is just local markdown plus `[[wikilinks]]` plus a graph view; it is popular because **the files are the data**,
and any agent can read and write them with no API. In 2026 a wave of "let the agent maintain the vault" templates appeared, and obsidian-mind
is the most structurally complete: notes live in folders by purpose (decision records, incidents and postmortems, people and teams,
observed patterns, pitfalls, workflows, references, drafts), each folder has a MOC index, a note "lives in one folder and
links to many places", and the rule is **a note with no links is a bug**. It runs on hooks: every user input is classified first
(decision / incident / win / meeting / person / project) and then given a routing hint; after every write, links and frontmatter are validated,
and an oversized note prompts a split; there is a routine command each for end of session, weekly, and audit. Supports Claude Code, Codex, Gemini;
other agents read the conventions only. ([obsidian-mind](https://github.com/breferrari/obsidian-mind) ·
[overview](https://www.stefanimhoff.de/writing/agentic-note-taking-obsidian-claude-code/))

Comparison: what is worth taking is not Obsidian but three disciplines: **classify before routing on intake** (in our candidate pool
HQ assigns the topic by hand at intake, with no pre-classification); **validate right after writing** (our render is deterministic but does not validate links);
**periodically audit orphans and stale entries** (same, lint is missing). Its note-type table also confirms the two kinds we lack:
decision records, and "observed patterns", the judgment basis that is neither a workflow nor a pitfall.
Another cheap win: the supervisor's `knowledge/*.md` is already markdown with `[[links]]`,
so **a user can open the supervisor's home in Obsidian today and see the graph**, with no work on our side.

### 3. Zettelkasten and Evergreen notes

Luhmann's slip box and Matuschak's evergreen notes are one line of thought: **one note, one concept** (atomic),
**organized by concept rather than by source** (not by book, by project, or by who said it), **densely linked**, and explicitly
"prefer associative ontologies to hierarchical taxonomies". Evergreen means a note is continually revised rather than written and filed.
([Evergreen notes](https://notes.andymatuschak.org/z5E5QawiXCMbtNtupvxeoEX))

Comparison: our entries are atomic, linked and supersedable, so all three match. One thing is the exact opposite:
the `corrections` topic is **classified by source** ("what the commander corrected"), an anti-pattern in a Zettelkasten;
what a correction distills into may be a pitfall or a workflow. The source should be a field on the entry, not the drawer it lives in.

### 4. PARA and Johnny Decimal

PARA has four tiers by actionability: projects, areas, resources, archive. Johnny Decimal numbers folders.
Both are **for people managing work and files**; the question they answer is "which of my current concerns does this relate to".
([comparison](https://crystaljjlee.com/blog/two-approaches-to-pkm/))

Comparison: not borrowed. The supervisor's ledger is not task management; its entries are not archived when a project ends. "Archive" here
is called retire, and its criterion is "no longer holds", not "the project is over".

### 5. A-MEM: a Zettelkasten for agents (2025)

One memory has seven fields: original text, time, keywords, tags, a contextual description, a vector, links. When a new memory arrives,
the top-k nearest neighbours are fetched by vector, then the model judges whether there is a real connection; **and the neighbours are updated in return**: their
descriptions and tags are recomputed so that "higher-order patterns" grow out of the links. The paper's argument is exactly the Zettelkasten's three points:
atomic, flexibly linked, no preset hierarchy. 1200 to 2500 tokens per interaction against a 16900 baseline.
([paper](https://arxiv.org/abs/2502.12110) · [code](https://github.com/agiresearch/a-mem))

Comparison: two things to borrow, one not. Borrow **finding neighbours before intake**: when a candidate arrives, list the most similar ledger entries,
which is precisely the capability the supervisor asked for in the first mining batch ("the expensive part is seeing which 11 of the 146 are one thing");
and **a new entry triggering updates to old ones**: we have supersede, but no "the linked entry writes back to its family".
Vectors are not borrowed: gtmux is a cgo-free local CLI; start with topic + keyword overlap for neighbours and revisit when that stops being enough.

### 6. CoALA: the standard three-way split of agent memory (2023)

Episodic memory (what happened), semantic memory (how the world is), procedural memory (how things are done),
plus working memory (the current context). This is now the default frame of reference in agent-memory papers.
([paper](https://arxiv.org/pdf/2309.02427))

Comparison: this answers the "kind" axis. Episodic memory for us is the event stream and the situation board, which never enter the ledger;
the ledger holds the semantic and procedural kinds. The six existing topics fall into place: environment / accounts are semantic,
workflows / pitfalls / best-practices are procedural (one positive, one negative, one optimization), and corrections is not a kind.
The missing kinds: decisions (why A over B) and judgment basis (how to judge in which situation); the former leans semantic in CoALA,
the latter procedural, and both are scattered across PRs and proposals today.

### 7. ACE: an evolvable playbook and its two failure modes (2025)

Three roles: a Generator produces trajectories, a Reflector extracts lessons from them, a Curator merges the lessons incrementally as **entries with ids and
helpful / harmful counts**, and the merge is deterministic. It names two failures: **brevity bias**
(every optimization pass shortens, deleting domain detail and failure modes) and **context collapse** (letting the model rewrite the whole thing:
18282 tokens collapsed to 122 in one step, and accuracy fell with it). ([paper](https://arxiv.org/html/2510.04618v1))

Comparison: our distill is the Curator, mining is the Reflector's input, and that line is already wired in `hq-transcript-mining`.
The two failure modes must become hard constraints on distill: **incremental only, never a whole-file rewrite** (render
is deterministic and supersede keeps the old text; both already defend this); the "helpful / harmful count" corresponds to our recurrence feedback
and should become a field on the entry.

### 8. Agent Skills: packaging procedural knowledge with progressive disclosure

SKILL.md packages "how to" as a loadable directory: at session start only each skill's name and a one-line description
(about 100 tokens) enter the context; the body and attached files are read on demand. This design is now an open standard.
([docs](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview) ·
[analysis](https://www.newsletter.swirlai.com/p/agent-skills-progressive-disclosure))

Comparison: this directly decides how "machine-wide rules" are distributed. **Embedding the full text** in every agent's global instruction file
bloats over time and crowds every session's context; the right shape is the Skills one: the managed block in the instruction file carries only an
**index** (one sentence per entry plus the path to the master copy), the full text stays in gtmux's master copy, and the agent reads it when needed.
Codex, opencode and Kimi do not support `@` imports, but all can read files, so index + path works for all four.
A second implication: if the `workflows` kind in the ledger grows into a multi-step procedure, its destination may be a skill,
not a markdown entry.

### 9. SECI: the spiral of tacit knowledge

Nonaka's four steps: socialization (tacit to tacit), externalization (tacit to explicit), combination (explicit to explicit), internalization (explicit to tacit).
([SECI](https://en.wikipedia.org/wiki/SECI_model_of_knowledge_dimensions))

Comparison: not a tool, but the name of this loop. Mining and corrections are externalization, distill's merge is combination, distribution into each
agent's instruction file is internalization, and recurrence feedback is the signal of "internalization failed". It is a reminder of one thing: **internalization
is the goal**; the three steps before it only serve it. So "distribution" is not an optional wrap-up, it is the reason the loop exists.

## After the comparison: what to borrow, what not

| Mechanism | Source | Do we have it | Judgment |
|---|---|---|---|
| Atomic entries, by concept not by source | Zettelkasten / Evergreen / A-MEM | entries are atomic; `corrections` by source is an anti-pattern | borrow: source demoted to a field |
| Links over hierarchy | same | `[[family]]` links exist; topics are a single level | borrow: topic as the primary class, cross-topic via links |
| Immutable raw + citations back to raw | Karpathy | seq provenance; no excerpt kept on intake | borrow: an entry keeps the conversation excerpt that triggered it as an example |
| Compile rather than retrieve | Karpathy | already | have |
| Lint / periodic audit | Karpathy / obsidian-mind | none | borrow: `gtmux knowledge lint`, folded into self-check |
| Find neighbours before intake | A-MEM | none | borrow: list similar entries at intake for HQ to judge a merge |
| New entries write back to old ones | A-MEM | only supersede | borrow: update the linked entries' family at distill |
| Helpful / harmful count | ACE | recurrence count lives in the mining ledger, not on the entry | borrow: becomes an entry field |
| Incremental only, no whole-file rewrite | ACE | deterministic render, supersede keeps the old | have; write it as a hard constraint |
| Progressive disclosure | Skills | none | borrow: distribution carries only index and path |
| Classify before routing on intake | obsidian-mind | HQ judges by hand | partly: mining candidates can be pre-tagged with a kind |
| Three-way memory split | CoALA | topics roughly correspond | borrow: as the "kind" axis |
| Tiers by actionability | PARA / Johnny Decimal | none | not borrowed |
| Vector neighbours, graph database | A-MEM / Zep | none | not borrowed: topic + keywords first |
| Obsidian as UI | ecosystem | the ledger is already markdown with wikilinks | nothing to do; tell users they can open it directly |

## The three axes this settles, and where they change the status quo

One piece of knowledge occupies one slot on each of three orthogonal axes, so that the supervisor and a human can both answer "what is this, where from, for whom":

1. **Kind** (CoALA's split, one level finer): `facts` (environment and accounts merged in), `howto` (workflows),
   `pitfalls`, `judgment` (judgment basis; best-practices merged in), `decisions`. The ledger's topic vocabulary
   was always extensible; this changes the built-in vocabulary, not the system.
2. **Provenance** (becomes a field, no longer a topic): from a correction, a recurrence, mining, or self-report; with a count. This slot is
   ACE's count and SECI's "internalization failed" signal.
3. **Audience** (distribution scope): supervisor / this machine / repo / everyone. See the four landing spots in `knowledge-layers.md`.

Plus one state, `hypothesis`: a lead from mining not yet confirmed gets a place, so it is neither forced into the ledger nor lost.

## Explicitly not done

- No vector store or graph database. Neighbours use topic + keyword overlap first; revisit when it is not enough, and only with measured evidence.
- No Obsidian as a dependency. The ledger is already in a shape it can open; that is free.
- No PARA-style archiving. Retire's criterion is "no longer holds", not "the project ended".
- Distill never rewrites a whole topic file. That is the collapse path ACE's experiments proved.

## References

- Karpathy LLM Wiki: [concept overview](https://denser.ai/blog/llm-wiki-karpathy-knowledge-base/) · [one implementation](https://github.com/Astro-Han/karpathy-llm-wiki)
- obsidian-mind: [repository](https://github.com/breferrari/obsidian-mind)
- Evergreen notes: [Matuschak](https://notes.andymatuschak.org/z5E5QawiXCMbtNtupvxeoEX)
- PKM comparison: [PARA vs Zettelkasten](https://crystaljjlee.com/blog/two-approaches-to-pkm/)
- A-MEM: [arXiv 2502.12110](https://arxiv.org/abs/2502.12110)
- CoALA: [arXiv 2309.02427](https://arxiv.org/pdf/2309.02427)
- ACE: [arXiv 2510.04618](https://arxiv.org/html/2510.04618v1)
- Agent Skills: [Claude docs](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview)
- SECI: [Wikipedia](https://en.wikipedia.org/wiki/SECI_model_of_knowledge_dimensions)
- See also the earlier comparison: the judgment on the claude-mem, episodic-memory and mem0 lineage in the `hq-transcript-mining` proposal
