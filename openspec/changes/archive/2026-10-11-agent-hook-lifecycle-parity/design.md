# Lifecycle evidence and upgrade safety

Normal completion, user interruption and terminal failure all end an active turn, but only normal completion earns a completion notification. Store a non-success outcome next to pane state (and in a native record) so the server's independent poller preserves that distinction. Clear the outcome on the next actual turn or session; queued prompts and subagent/tool failures do not finish the parent turn.

Codex Interrupt is an observation-only main-thread event with a maximum three-second hook budget. It must use session/cwd ownership rather than trusting an app-server's inherited TMUX_PANE. Rollout turn_aborted is a fallback only when newer than the active and waiting markers and belongs to the bound session. A newer task_started wins. Screen text in scrollback alone is never cancellation evidence.

Doctor checks the installer actually selected for an agent. JSON hooks must carry gtmux's expected event command in an executable handler; another tool's handler does not satisfy it. Kimi's managed TOML block remains byte-preserving. opencode's dedicated plugin is compared with the current generated bridge. Updates back up configuration, preserve foreign hooks and never restart a live agent or grant trust automatically.

Upstream events without a parent-turn lifecycle meaning are deliberately excluded. Agents without a documented cancellation hook keep their existing screen/process fallback; no invented Interrupt key is written. The audit records that limitation explicitly.
