# Tasks: tiered-pane-control

Sequenced so each PR is independently shippable; the CLI producer lands first and
unblocks every surface.

## 1. CLI producer — `gtmux panes` (PR 1)

- [x] 1.1 `internal/radar`: `GatherPanes()` enumerates all tmux panes (one
      `list-panes -a`) into a `PaneRow` with id, loc, session/window/pane, cwd,
      command, title, active, inMode, and `tier` (`agent`|`plain`). `tier` reuses
      `classifyAgent` / cross-references `GatherAgents` so an agent pane is marked and
      linkable. Read-only, off the radar's 1.5s poll. Tests: fixture panes → tiers,
      agent vs plain, `agents --json` unchanged.
- [x] 1.2 `internal/app`: dispatch `panes` → `--json` array + plain-text session tree.
      Register in `app.go`.
- [x] 1.3 Docs surface (same PR): CLAUDE.md command list, `gtmux --help` (en+zh),
      `## gtmux panes` in `docs/cli.md`, and note the `tier` field. (No HTTP endpoint
      yet → no api/contract.md change; add when the app needs it.)

## 2. Tier contract + non-agent focus (PR 2, partly landed)

- [x] 2.1 `gtmux focus %pane` states when the target's agent has exited (plain shell
      now) and still jumps; a gone pane reports missing. (Shipped in the papercut PR.)
- [x] 2.2 `docs/cli.md` + help: document that `focus`/`send` take any pane id (not
      just agents) and the tier capability matrix (agent = full, plain = focus/type/
      watch/attach, sensed non-tmux = read-only).

## 3. Watch-this-pane (PR 3)

- [x] 3.1 `internal/state`: a `watched/<pane>` marker set (add/remove/list),
      auto-reaped by the existing orphan sweep when the pane closes. Tests.
- [x] 3.2 `internal/radar`: `GatherAgents` appends watched plain panes as a distinct
      `source:"watched"` (or a `watched:true` flag) row — NO agent status, marked as
      watched. Never auto-added. Tests: watched pane present + distinct; unwatched
      plain pane absent; closed watched pane dropped.
- [x] 3.3 `gtmux watch <pane>` / `gtmux watch --remove <pane>` / `gtmux watch --list`
      CLI (or fold into `panes`), documented per the command-docs rule.

## 4. Menu-bar surfaces (PR 4, Swift)

- [ ] 4.1 A sessions/panes browser view (consumes `gtmux panes --json`): session →
      window → pane tree, focus/type/attach any pane, agent panes badged + linking to
      their Detail. A SEPARATE view, reached from the ⚙ menu / a browser affordance —
      NOT merged into the radar list.
- [ ] 4.2 Agent-neighbor strip in an agent's Detail: sibling panes in the same
      session (filtered `panes --json`) with focus/type.
- [ ] 4.3 "Watch this pane" affordance in the browser; watched panes render as a
      distinct radar row type (own glyph, no agent status).
- [ ] 4.4 DESIGN.md: a NEW section for the pane browser, kept separate from the radar
      section, stating the anti-dilution rule (plain panes never auto-enter the radar).

## 5. Mobile / web (PR 5, optional follow-up)

- [ ] 5.1 Generalize the phone/web surfaces to the pane browser + neighbor panes using
      the same `panes --json` + `send`/`attach` contract; MOBILE.md / WEB.md notes.
      Guest scope unchanged. (Can trail the desktop surfaces.)

## 6. Close-out

- [ ] 6.1 `openspec validate --specs --strict` green; sync deltas into
      `openspec/specs/{pane-browser,agent-radar,terminal-jump}` and archive the change.
- [ ] 6.2 Positioning check: help/docs frame plain-pane reach as "you're never stuck,"
      not "tmux manager"; the README's five-ways-in is unchanged (this is a capability,
      not a new surface headline).
