## Context

gtmux classifies every tmux agent as waiting/working/idle/running via two paths
that merge in `gatherAgents` (`internal/app/agents.go`): a **hook path** that
writes cross-process state markers under `~/.local/share/gtmux/`, and a **radar
scan** that each poll reads those markers plus process-tree CPU and screen
frames. There is one existing *modifier* on a state: `error`/`error_text`, set on
`idle` rows whose Claude transcript ended on an API error, rendered as an amber ⚠
that overlays the idle glyph without changing the status. That modifier is the
template this change follows end to end.

New fact established by primary-source verification (the installed Claude Code
2.1.205 binary's Zod schema): the `Stop` (and `SubagentStop`) hook payload carries
`background_tasks: array<{ id, type, status, description, command? }>`, documented
verbatim as *"In-flight background work (running/pending + backgrounded)
registered in this session. Lets hooks distinguish 'session is done' from 'session
is paused waiting for background work to wake it'. Empty array when nothing is in
flight."* `type` is a label like `shell`/`subagent`/`monitor`/`workflow`;
`command` is present only for `shell` items. gtmux's Claude hook already decodes
the `Stop` stdin JSON (`internal/hook/hook.go`), so reading one more field is
cheap. Codex/other agents expose no equivalent — confirmed out of scope for v1.

## Goals / Non-Goals

**Goals:**
- Read Claude's official `background_tasks` at `Stop` and record per-pane whether
  the settled session still has in-flight background work (+ count + a short
  label).
- Surface it as a `bg` modifier on `idle` in `gtmux agents --json`, mirroring
  `error`, and render it on all four surfaces (CLI, menu-bar, mobile, Web) in an
  amber/neutral tone, never red.
- Close the pre-existing gap where the Web mirror renders neither `error` nor the
  new `bg` modifier.

**Non-Goals:**
- No new status; `bg` only annotates `idle`.
- No screen-scraping, no OS process-tree probing in v1.
- No Codex/other-agent support; no `session_crons` surfacing.

## Decisions

### D1 — Signal source: Claude `Stop` payload `background_tasks`, not process tree

Use the agent's own official end-of-turn payload. Rationale: it is structured,
purpose-built for exactly this "done vs paused-for-background-work" distinction,
zero false positives (it lists only work *this session* registered), and carries
a human label (`command`). Alternatives considered: (a) OS process-tree
(subtreeCPU / live descendant) — agent-agnostic but false-positive-prone (ambient
dev servers/LSPs the user started manually look identical) and would need extra
filtering; deferred as a possible Codex fallback. (b) Screen-scraping — rejected;
the survey (AgentAPI #207, claude-squad) shows it is brittle and breaks on agent
UI changes. The hook payload is the "梯队 A" best-practice signal.

### D2 — Marker: a new `bg/<pane>` state file, mirroring `finished/<pane>`

At `Stop`, after computing the existing `finished` marker: if `background_tasks`
has ≥1 running/pending item, write `bg/<pane>` whose contents encode the count and
a short label (e.g. first shell `command`, else first item `description`);
otherwise remove `bg/<pane>`. All lifecycle transitions that clear `active`/
`finished` (UserPromptSubmit, SessionStart/End, staleStop) also clear `bg/<pane>`
so it can never outlive its turn. `internal/state` gains read/write/clear helpers
alongside the finished-marker ones. Rationale: markers are the established
cross-process contract between the hook and the radar; reusing the pattern keeps
the two paths decoupled and the marker naturally self-heals on the next turn.

### D3 — Radar surfacing: `bg`/`bg_count`/`bg_text` on `agentJSON`, idle-only overlay

In `gatherAgents`, only for rows whose final status is `idle`, read `bg/<pane>`
into `agentPane.bg/bgCount/bgText` → `agentJSON.bg/bg_count/bg_text` (JSON keys
`bg`,`bg_count`,`bg_text`), exactly parallel to `error`/`error_text`. CLI render
overlays an amber `⧗N` + label on the idle glyph (like the `⚠` overlay), status
unchanged, i18n en+zh. Rationale: one code path, one visual grammar, no new
status rank, honours the "red only for waiting" 铁律.

### D4 — v1 producer scope gated to Claude

Only the Claude `Stop` branch writes `bg/<pane>`. Codex's `agent-turn-complete`/
`Stop` carry no background field, so its rows never get the marker — no special
casing needed, the absence is the correct behaviour. A later change can add a
process-tree fallback behind the same contract fields.

### D5 — Fix the Web `error`/`bg` rendering gap in the same change

`internal/server/web/app.js` currently buckets purely by `status` and ignores
`error`/`error_text`. Since we are teaching every surface the `bg` modifier,
render both `error` (existing, currently missing) and `bg` there so the Web
mirror reaches parity. Small, contained, and avoids shipping a second modifier
the Web surface silently drops.

## Risks / Trade-offs

- **[`background_tasks` is undocumented on the public docs page (schema lives in
  the binary); Anthropic could rename/remove it]** → Treat it as best-effort:
  decode defensively (optional field, tolerate absent/renamed → simply no `bg`),
  never let a missing/changed field break `Stop` handling. Add a hook unit test
  with a captured sample payload so a shape change is caught by `make check`.
- **[Only Claude reports it → inconsistent across agents]** → Acceptable and
  documented as a non-goal; the modifier is purely additive, so a Codex idle row
  looks exactly as it does today. No regression.
- **[A background task that outlives many turns keeps the row marked]** → Correct
  by design: as long as the work is in flight the session genuinely is paused for
  it; the marker clears the moment a later `Stop` reports an empty array.
- **[`bg_text`/`command` could be long or contain odd characters]** → Cap length
  (Claude already caps at 1000; we cap tighter for a one-line label) and treat as
  display-only text, never executed.
- **[Contract addition rippling to 4 consumers]** → Additive optional fields;
  each consumer added independently. Menu-bar/mobile/Web are pure consumers, so a
  staged rollout (CLI first) never breaks an older consumer.

## Migration Plan

Additive only — no migration. Order: (1) hook + state marker + CLI render + tests
land together (self-contained, `make check` green); (2) `agents --json` gains the
optional fields; (3) menu-bar, mobile, Web consumers render the modifier. Rollback
is trivial: the fields are optional and ignored by any consumer that predates
them; removing the producer simply stops emitting `bg`.

## Open Questions

- Glyph choice for the modifier (`⧗` hourglass vs `⏳`/`◔`) — pick one that renders
  cleanly in the CLI (respecting the past U+FE0E/emoji-width traps) and matches
  DESIGN.md's restraint; finalise during CLI render implementation.
- Whether to show the count (`⧗2`) or just the mark (`⧗`) when `bg_count > 1` on
  the space-constrained menu-bar/mobile rows — default to showing the count; drop
  to bare mark only if width forces it.
