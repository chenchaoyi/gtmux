# Tasks — session-events

- [ ] 1.1 `internal/events`: Append(record) with SIZE-TRIGGERED ROTATION (rename
      active→events.1.jsonl at a cap, keep K generations, total bounded; rename is
      atomic-ish, O_APPEND single-line writes). Record = {ts,event,state,pane,loc,
      session,agent,kind}. Unit tests: append, rotate-at-cap, generation-pruning,
      concurrent-append integrity.
- [ ] 1.2b Read(since) across generations + Follow with tail -F semantics
      (re-open on rotation/inode-change so following never stops). Test rotation
      mid-follow.
- [ ] 1.3 hook: append one record per event after decide()/applyState — additive,
      never blocks the hook; native (no pane) events included.
- [ ] 2.1 `gtmux events [--follow] [--json] [--since <dur>]`.
- [ ] 3.1 HQ playbook + knowledge: `gtmux events --follow` as the subscription
      (when to tail vs snapshot digest). supervisor-agent spec delta.
- [ ] 3.2 Docs (cli.md) + CLAUDE.md contract note; sync-specs + archive.
- [ ] 4.1 make check green; dogfood: tail shows live multi-session events; HQ uses it.
- [ ] 5.1 (P2) per-session activity ring in the apps; consumer filters.
