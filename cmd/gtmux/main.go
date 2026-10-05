// Command gtmux is a command center for tmux sessions and the agents in them.
// All logic lives in internal/app; this entry point just hands off os.Args.
package main

import (
	"os"

	"github.com/chenchaoyi/gtmux/internal/app"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/dispatchbridge"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func main() {
	// Before anything can create a file: everything gtmux writes is readable by its
	// owner only (openspec change `diagnostics`). This runs for every invocation — the
	// hook, serve, the tunnel client, a command — because each of them writes state.
	state.PrivateUmask()
	// A dispatch ledger entry names its pane by tmux ID, and IDs start over with each tmux
	// server: entries from before this server's start must not attach to today's panes.
	// Set here, for the real process only, so tests keep their small fixed timestamps.
	dispatch.PaneEpoch = dispatchbridge.TmuxServerStart
	os.Exit(app.Run(os.Args[1:]))
}
