# Create a Mac tmux session from a paired phone

## Why
The phone can control existing panes but cannot start a new working space. A user away from the Mac must first create one locally, even though menu-bar New already provides this capability.

## Design
An owner-only New session action lives in the radar and All panes headers, with a labelled empty-state action. A focused name form identifies the active Mac, accepts an optional name and says Create and open. Success dismisses the form before opening the returned plain pane in Terminal via the shared workspace (phone stack / iPad main canvas). It creates a detached shell in the Mac user's home without activating a desktop terminal. Mac New retains its existing create-and-open-tab behavior through a shared core helper.

Use authenticated POST /api/sessions with a client request ID. Core creation serializes the check/create path within the serving process. Atomic tmux session environment tags retain the request ID and normalized-name hash while the session is live, allowing response-loss retries and serve restarts to find the same session. A repeat ID with changed arguments is refused. No agent, command, path, deletion or desktop activation parameters are accepted. A network uncertainty retains the request ID and name; it offers retry or a session-list check. A guest is refused server-side and sees no creation controls; offline controls are disabled. An old server produces an explicit update instruction.

## Surfaces
- phone: radar/All panes entry, keyboard-ready bounded sheet, inline errors and direct terminal opening.
- iPad: same sheet, target Mac visible, new pane selected in the existing main canvas.
- Terminal: shared detached creation helper; CLI New still opens a local terminal tab.
- menubar: existing prompt/CLI invocation unchanged.
- Web: no new UI; authenticated API contract gains an owner-only operation.

## Limits
Request identity persists only as long as the tagged tmux session survives. Separate serve processes are not a cross-process transaction coordinator. No agent-launch presets or remote folder picker in this batch.

## Validation
Real isolated tmux tests for creation, naming, default directory and request replay, injected HTTP tests for authorization/validation/errors, mobile API/form/navigation tests, full repository/mobile/design gates, and cached-debug simulator UI checks if reusable. No release or installation is included.
