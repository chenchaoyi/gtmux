## 1. Hook: read Claude `background_tasks` and record the marker

- [x] 1.1 In the Claude `Stop` stdin-JSON decode (`internal/hook/hook.go`), parse
  the optional `background_tasks` array (items: `id`, `type`, `status`,
  `description`, `command?`) defensively — absent/renamed field ⇒ empty, never an
  error.
- [x] 1.2 Compute in-flight count = items whose `status` is running/pending
  (tolerate unknown status strings by counting non-terminal), and a one-line
  label (first `shell` item's `command`, else first item's `description`), capped
  to a short display length.
- [x] 1.3 In `decide()`/`applyState`, on `Stop`: if count ≥ 1 write the `bg`
  marker, else clear it. Ensure every lifecycle that clears `active`/`finished`
  (UserPromptSubmit, SessionStart, SessionEnd, staleStop, Resumed) also clears the
  `bg` marker.
- [x] 1.4 Add `internal/state` helpers `WriteBackground/ReadBackground/ClearBackground`
  (path `~/.local/share/gtmux/bg/<pane>`), mirroring the finished-marker helpers;
  document the path in the `state.go` header contract comment.

## 2. Radar: surface `bg` on the JSON contract (idle-only)

- [x] 2.1 Add `bg bool`, `bgCount int`, `bgText string` to `agentPane`
  (`internal/app/agents.go`) and `bg`/`bg_count`/`bg_text` (omitempty) to
  `agentJSON`, parallel to `error`/`error_text`.
- [x] 2.2 In `gatherAgents`, only for rows whose final status is `idle`, read the
  `bg` marker into those fields. Never set `bg` on working/waiting/running rows.
- [x] 2.3 CLI render (`agents.go` status/label rendering): when `bg` is set,
  overlay an amber/neutral `⧗N` mark + label on the idle glyph, keeping status
  `idle`; add en+zh strings via `internal/i18n` ("background running" / "后台运行中").

## 3. Tests

- [x] 3.1 Hook unit test: feed a captured Claude `Stop` payload with a
  `run_in_background` shell in `background_tasks` → asserts the `bg` marker is
  written with the right count + label; empty array → marker cleared; a later
  empty-array `Stop` clears a prior marker.
- [x] 3.2 Classifier/radar test: an idle pane with a `bg` marker yields
  `bg:true`+`bg_count`+`bg_text` in `agents --json`; a working/waiting pane with a
  stray marker does NOT; a non-Claude/idle pane without a marker is unaffected.
- [x] 3.3 `make check` green (gofmt + go vet + staticcheck + go test -race), and
  `CGO_ENABLED=0 go build ./cmd/gtmux` passes.

## 4. Consumers: menu-bar, mobile, Web

- [x] 4.1 Menu-bar (`macapp/Sources/GtmuxBar/AgentStore.swift`): decode
  `bg`/`bg_count`/`bg_text`; render the modifier per `docs/design/DESIGN.md`
  (amber/neutral, idle glyph kept, never red). `swift build -c release` passes.
- [x] 4.2 Mobile (`mobileapp/src/api/types.ts` + `src/ui/AgentRow.tsx` + theme):
  add the fields; render "idle · ⧗N 后台运行中" per `docs/design/MOBILE.md`;
  `cd mobileapp && npm run check` passes.
- [x] 4.3 Web (`internal/server/web/app.js`): render the `bg` modifier AND fix the
  pre-existing gap by rendering `error`/`error_text` too, matching the other
  surfaces (amber/neutral, not red).

## 5. Spec sync & docs

- [ ] 5.1 After implementation, `/opsx:sync` (or `sync-specs`) the delta into
  `openspec/specs/agent-radar/spec.md`; `npx @fission-ai/openspec validate --specs
  --strict` passes.
- [ ] 5.2 Note the new `bg*` fields in any JSON-contract doc/`CLAUDE.md`
  contract list if present; ensure `docs/design` "code位置对照" stays accurate.
