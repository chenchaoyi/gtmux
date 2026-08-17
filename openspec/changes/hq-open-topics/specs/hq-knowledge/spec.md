# hq-knowledge (delta)

## ADDED Requirements

### Requirement: The topic vocabulary is extensible through the ledger

The topic vocabulary SHALL be the built-in topics plus every topic DECLARED in
the ledger: `gtmux knowledge topic <name> --desc "…"` appends a `topic`
operation (name + one-line description), a mutation under the knowledge role
gate that journals through `gtmux:audit:knowledge` like every other. A topic
name SHALL be a bounded slug (`[a-z0-9-]`, ≤ 40 bytes) and SHALL refuse —
loudly — a built-in, an already-declared custom, and the reserved names the
directory layout owns (`README`, `legacy`, `promotions`).

One validation over one source SHALL serve every entrance: `gtmux knowledge`
verbs and `gtmux capture` accept exactly the same vocabulary (built-ins,
`environment`, and declared customs — closing the drift where capture accepted
five topics and knowledge six). A declared topic SHALL render immediately
(marker, name, its description as the intro), so the declaration is visible
rather than a silent registry write, and the dispatch-time knowledge echo SHALL
consult every CUSTOM topic alongside the built-in pitfalls/workflows — the
built-in exclusions (accounts, corrections, environment: not dispatch-time
context) stand unchanged. Topic declarations are add-only; removal is deferred
until a real need names it.

#### Scenario: A user's own domain becomes a first-class topic

- **WHEN** HQ declares `gtmux knowledge topic datasets --desc "…"`, a worker runs
  `gtmux capture "… @datasets"`, and HQ accepts the candidate into an entry
- **THEN** the ledger holds the declaration and the entry with its provenance,
  `datasets.md` renders with the description and the lesson, and a later
  dispatch whose repo or goal matches surfaces it in the knowledge echo

#### Scenario: The vocabulary refuses nonsense loudly

- **WHEN** a topic declaration names a built-in, an existing custom, a reserved
  directory name, or an over-long/invalid slug
- **THEN** it is refused with the reason named, and nothing is appended

#### Scenario: Capture and knowledge can no longer disagree

- **WHEN** any topic is judged by `gtmux capture` and by `gtmux knowledge add`
- **THEN** both entrances give the same verdict, from the same vocabulary
