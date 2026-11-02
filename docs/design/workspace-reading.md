# Workspace reading and human requests

The current question is separate from progress history. HQ uses `request-input` with
`matter`, `revision` and `user_input` (`id`, `kind`, `prompt`, `why`, `impact`, optional
`choices`). Kinds are `decision`, `manual`, and `information`. A request names one
concrete human action. Technical work, routine status and fulfilled permissions do
not belong in this queue. `clear-input` records a verified resolution using the
question ID in `reply_to`; it returns the continuing matter to follow-up, rather than
marking the goal complete. The original request and receipts remain in history.

The native workspace prepares a scoped draft when a choice is selected. Sending
uses the existing durable instruction path and `reply_to`. Duplicate submissions
converge; stale requests are rejected. A saved reply is not execution or confirmed
delivery. Cancelling an unsent reply restores the question. Mobile choices prepare
an ordinary HQ composer message; HQ reconciles that answer through the existing
channel after receipt. It must not infer permission from a draft or a selected UI
option. Old servers remain readable; unstructured records are shown for HQ
clarification, not as actionable questions. The legacy `needs_input` projection is
retained; new readers use `human_input` and `awaiting_hq`.

Browser Trace analysis and presentation are generated from the App's pure TypeScript
modules. Activity semantics and icon tables retain their existing conformance tests.
A conversation overview uses the same index, original turn ordinals and excerpts.
It covers the loaded window only. At most 120 marks are mounted; keyboard and pointer
navigation can reach every loaded turn. Touch previews remain open for the explicit
jump action. Refresh preserves history reading; no navigation gesture submits agent
input or fetches unshared conversation data.

Terminal spacing belongs to the xterm element, as FitAddon expects. Host observers
refit when controls, maximization or viewport changes alter available space. The
terminal keeps its own resize/reflow state rather than resetting its screen twice.
Observers and deferred fits are guarded when tiles close or switch modes.

Exact pane focus checks the requested live pane, selects its window and pane, and
brings forward the terminal. It does not scan the entire agent fleet merely to print
an optional warning before the visible jump. Native tab identity and terminal driver
selection retain the existing compatibility paths.
