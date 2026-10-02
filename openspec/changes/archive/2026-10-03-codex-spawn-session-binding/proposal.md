# Bind spawned Codex sessions using a delivery witness

## Why
A new Codex worker in the same cwd as an existing Codex pane cannot bootstrap its resume binding: the shared app-server hook has an unreliable inherited pane, and cwd ambiguity correctly refuses attribution. The real rollout is then surfaced as native while the tmux row has no Chat history. The observed worker has a 957 KB rollout and no resume record.

## Design
For an unbound interactive Codex spawn, persist a short-lived delivery intent with a random token, SHA-256 of the complete wire prompt, target pane ID/location/PID/cwd and the incumbent binding. Append a reserved metadata line to the task. A submitting hook can bind only the exact wire prompt to the unchanged live target, with the real terminal-originator session ID. Polling also reconciles a native rollout containing that exact submitted user message, covering missing hook IDs or late rollout persistence. Consume the intent only after writing the resume record. Existing bindings and unknown/desktop sessions do not acquire ownership by cwd, title or recency. Hide valid metadata lines in parsed Chat and mined prompts.

Intents expire after ten minutes; bounded rollout reads inspect the last 8 MiB. Failure to prepare an intent is logged and does not prevent delivery. No global Codex metadata, agent restart or runtime deployment is required by this code change. Historical workers without an intent require separately verified repair, not inferred automatic association.

## Surfaces
- terminal: spawn gains verified Codex binding; payload metadata is filtered from parsed user prose.
- menubar: the shared radar suppresses the duplicate native row once bound.
- phone: the shared backend makes Chat available and removes the duplicate; no app code changes.
- iPad: same shared backend and Chat behavior.
- Web: same transcript/radar data, no separate detection logic.

## Validation
Synthetic same-cwd peers, stale inherited panes, complete prompt mismatch, expired tokens, changed pane PID/location/cwd/incumbent, desktop originators, missing hook IDs, failed writes and native suppression/history recovery. Full make check and design gate; no large native build.
