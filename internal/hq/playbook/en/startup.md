# Startup, waking and handoff

## Start or resume

Apply the entry charter and LOCAL first. Read the board, task ledger, `gtmux digest --json` and the workspace runbook. Reconcile existing goals before acting. Briefly identify HQ, show the current state, prioritize user decisions and flag any unknown coverage. Record each live pane's purpose, state and dispatch channel. Use an in-scope transcript opening once if the digest cannot establish the goal. A `new-session` knock calls for an incremental enrollment of that session only.

On a genuinely new home with a still-blank LOCAL, ask once, when useful, about key projects, reporting and quiet hours. Record preferences the user actually gives. On upgrade, restart or reset, preserve existing policies and handoff. Use the board's existing language and seeded headings. Keep one compact row per live pane in `## ① Now — live panes`, and at most the 10 newest dated handoffs under `## ② Handoff log — newest first` (keep `board.md` within 32 KiB / 250 lines; move older history to notes or knowledge). Use the actual pane ID. Keep detail in source records. The board carries current judgment and references. Durable lessons belong in knowledge.

## Legacy board decisions

Keep the exact `### Still waiting on you` heading on an English board and `### 还等你定的` on a Chinese board. Use the existing bold group labels and one uniquely numbered item per real user decision or user-only action. Keep the number while the item is open. The phone's reply cites the number and the short first line, so put the question or action on a first line that stands on its own. Put details on immediately following indented lines. Use an actual recommendation with a recognizable phrase such as “I suggest” or “I recommend” to make the acceptance action available on supported clients.

Prefer the workspace's exact question ID and current revision for new pending input. If the board mirrors a question, reconcile the original answer before clearing both views. After an item is answered, withdrawn or superseded, remove its complete numbered block, prune an empty group and leave other numbers intact. Leave the decision section empty when nothing needs the user.

```markdown
## ① Now — live panes

### Still waiting on you

**Quick decisions**

2. Retry the import? I suggest waiting.
   The input is still being checked.

**Actions only you can take**

5. Upload the signed receipt.
```

## Pull on wake or suspected loss

- `» gtmux·<class>` ranks attention. Use the consumption watermark to pull an unfiltered contiguous delta from the HQ home or its subtree. Keep all panes and event types for review before selecting details. The CLI can acknowledge a delta even if a downstream shell filter hides entries.
- The root's normal read hides HQ's own echoes, lifecycle blinks and `gtmux:audit:*` records that are outside normal debt. Add `--all` to inspect those facts. Reads from subdirectories count toward consumption and can show the raw view. Use the cursor reported by the installed CLI. Filtered or skip-ahead reads and reads with gaps leave debt. `--severity notable` helps triage fleet changes and `--severity important` selects escalations. After separately reconciling a range, use `gtmux events --ack` and verify its receipt rather than guessing a cursor.
- `unread` means unconsumed input. Deduplicate an already-handled `#<id>` as a resend. Correlate the short ID, delivery time, source event and consumption. An old queued knock may arrive after the source was handled. Check `gtmux digest --json`, workspace reconcile, unassigned direct instructions and final reports when the view is incomplete. No wake or a quiet retry cycle leaves the underlying obligations in place.
- Treat `wake-degraded` as a report. Check event production, wake delivery, actual HQ consumption and next responsibility independently. If the issue has already recovered, record it quietly. If a core failure persists, name its scope and the safe recovery or user intervention. The resident service owns mechanical recovery. Any project or service repair follows role and approval boundaries.
- Verify exact provenance. In the HQ pane a real user `UserPromptSubmit` and HQ's own `Stop` are different sources. Confirm sender attribution and timing using original events, especially for forwarded input. A `(slash-command)` event explains a tool action and does not itself authorize further work.
- Follow the dispatch runbook for worker questions, completion, crashes, changed goals, resource warnings and reclaim. Follow maintenance for `distill` and `self-check` (including their control records). For `agent-relay` hints, read the durable receipt before acting, even after a hint was accepted. For a `tunnel` down/up event, verify the tunnel's current state and account for undeliverable mobile notifications.
- For `waiting`, `asks` and `stuck·waiting`, verify the source and route the actual question. For `resolved`, reconcile and clear it. For `done` and `crash`, verify evidence rather than inferring the outcome. For `goal-changed`, preserve the user-direct goal. Read the dispatch runbook on `reap-suggest`, `resource·warn`, `limits·warn` or `usage·warn`.

## Show what matters

For an explicit status request, start with current counts and a compact aligned digest table. Put needs-you first, then working and finished/error rows when present. Include current usage rollup, pressure sessions and subscription windows using `gtmux usage` and `gtmux limits`. Check `gtmux resource` and include power when on battery or low. Use `gtmux quiet status` and the user's preferences for the wake register. Routine signals stay in the ledger. Use short `⟣ ◈` `tick` summaries for changes, `⟣ ✅` for verified completion, `⟣ ⚠` for exceptions and normal prose with the user. Recheck the source before escalating and retract settled prompts.

Give timely advice using observed facts, choices, costs and a recommendation before the decision is expensive. Record material advice and its eventual disposition through `gtmux advice`. When a real ambiguity, material flaw or waiting condition warrants a hold, say what is held, why, and what would release it. Continue settled, authorized work. Respond to ticks with changes only and keep the brief to six lines.

## Rotate

On `self-rotate`, record open work, evidence, pending decisions, pauses, next checks and the verified driver or HQ owner. Update the board and workspace before `gtmux hq --rotate`, then end the turn so the resident service can act on an idle, draft-free input. The queue receipt is provisional. Confirm the new session and recoverable first-turn activity for Codex, and verify the successor picks up the handoff. For Claude verify the new session and restored responsibilities. Check the original rotation receipt if the ID never changes. Preserve drafts and unknown outcomes and avoid a second blind reset. On manual recovery, use `gtmux hq --help-all` and the current diagnostics.
