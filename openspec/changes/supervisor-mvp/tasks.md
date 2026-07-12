# Tasks — supervisor MVP

## 1. Digest layer (`internal/digest` + CLI)

- [ ] 1.1 `internal/digest`: assemble digest rows by joining the existing stores —
      radar gather (identity/state/since/errored/bg, incl. native rows), transcript
      (goal = last user prompt, last = reply tail; ride the incremental loader),
      waiting kind + `prompt.ParseOptions(capture-pane)` for ask, gitInfo for
      project/branch. Pure assembly; every field degrades to "" when its source is
      absent. Unit tests over fixture stores (no live tmux needed).
- [ ] 1.2 `gtmux digest`: human output (one compact block per agent, en+zh labels via
      i18n, radar ordering — needs-you first); `--json` machine output. Help text.
- [ ] 1.3 Keep `CGO_ENABLED=0 go build ./cmd/gtmux` green; zero new deps.

## 2. Digest over the API

- [ ] 2.1 `GET /api/digest` in `internal/server` (bearer-gated, additive route)
      returning the same array; handler test (auth + shape + a waiting row).

## 3. Supervisor session (`gtmux hq`)

- [ ] 3.1 `gtmux hq`: reuse-or-create the supervisor tmux session running the
      default agent profile in `~/.config/gtmux/hq/`; focus (existing terminal-jump
      path) when already live. Never spawn a second.
- [ ] 3.2 First-run seeding: generate the instructions CLAUDE.md (the digest→judge→
      drill→drive→report loop, toolbox: `gtmux digest --json` / `tmux capture-pane`
      / `gtmux send`; report-then-act norms; bilingual). NEVER overwrite an existing
      file (user edits + accumulated knowledge persist).
- [ ] 3.3 `role:"supervisor"` in `agents --json`: detect by pane cwd == hq home;
      additive omitempty field; JSON contract test.
- [ ] 3.4 Unit tests: seeding idempotence, reuse-not-duplicate decision (pure parts),
      role detection.
- [ ] 3.5 Waiting nudge (P1, user-promoted): on a Waiting decision in the hook, find
      a live hq pane (cwd == hq home, not the waiting pane itself) and send-keys one
      `[gtmux] waiting·<kind> <loc> — <title>` line + Enter; rides the existing
      notify dedup; `hqNudge:false` config disables; no hq → no-op. Tests for the
      pure decision (self-exclusion / dedup / config-off).

## 4. Docs + spec hygiene (per the consistency rule)

- [ ] 4.1 README(.zh) + docs/cli.md: `gtmux digest` + `gtmux hq` sections.
- [ ] 4.2 CLAUDE.md contracts line: add `/api/digest` + `role` field notes if needed.
- [ ] 4.3 On merge: sync-specs + archive this change (specs/ = built truth).

## 5. Gate

- [ ] 5.1 `make check` green; mobile untouched (no `npm run check` delta expected).
- [ ] 5.2 Dogfood: run `gtmux hq` against the user's live fleet; verify digest rows
      for ≥8 real sessions and a waiting session's ask text; supervisor answers
      "现状?" usefully from `digest --json` alone.

## 6. Deferred (P2/P3 — spec'd context, NOT built in this change)

- [ ] 6.1 P2: 中控 card on menu-bar/mobile fed by `/api/digest`.
- [ ] 6.2 P2: optional STATUS-block convention for cooperating agents.
- [ ] 6.3 P3: parallel-worktree orchestration; cross-model dispatch; multi-host mesh.
