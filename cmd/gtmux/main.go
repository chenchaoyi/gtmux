// Command gtmux is a command center for tmux sessions and coding agents.
// All logic lives in internal/app; this entry point just hands off os.Args.
package main

import (
	"os"

	"github.com/chenchaoyi/gtmux/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:]))
}
