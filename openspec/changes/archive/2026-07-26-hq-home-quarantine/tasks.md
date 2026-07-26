# Tasks: hq-home-quarantine

## 1. Spawn quarantine

- [x] 1.1 `spawnDirInHQHome`: refuse explicit `--cwd` = home, inherited cwd = home,
      and `--pane` whose current path = home (symlink-normalized via
      `hqpane.SameDir`). Bilingual refusal names `--cwd <project dir>` as the fix.
      Tests: explicit, inherited, and normal-project cases.

## 2. Identity precision

- [x] 2.1 `hqpane.resolve`: two-pass — any `@gtmux_hq_home`-stamped pane wins over
      the path fallback; fallback only when no stamp exists. Tests: worker parked in
      the home listed first no longer steals the identity; legacy no-stamp home still
      resolves.
- [x] 2.2 `hqpane.SameDir` exported — the one spelling of "is this the HQ home?".
- [x] 2.3 Radar: pane source carries `#{@gtmux_hq_home}`; `applyRolePrecedence`
      grants `role:"supervisor"` only to stamped panes when any stamp exists (native
      rows included in the withdrawal); `roleForCwd` becomes symlink-normalized and
      is only the no-stamp fallback. Tests: stamped-vs-worker, legacy fallback.

## 3. Charter guard

- [x] 3.1 Playbook: "身份自检 Identity check — READ FIRST" section + ALWAYS
      `--cwd` rule under the spawn tool; `hqPlaybookVersion` 10 → 11.

## 4. Docs

- [x] 4.1 `docs/cli.md` spawn section notes the HQ-home refusal.
