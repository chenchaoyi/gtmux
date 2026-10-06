package app

import (
	"os"
	"strconv"

	"golang.org/x/term"
)

// isTTY reports whether stdin is a terminal. It asks the terminal driver rather
// than the file type: /dev/null is a character device too, so a char-device
// check made `gtmux doctor < /dev/null` ask "Fix these now?" of an input nobody
// can type into (spec env-doctor: off a TTY it SHALL NOT prompt).
func isTTY() bool { return isTerminalFile(os.Stdin) }

func isTerminalFile(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

// termWidth returns stdout's current display width in columns, for the
// aligned table renderers (digest/usage/limits) to size their middle column
// and truncate long text. Falls back to $COLUMNS (set by most shells even
// over a pipe) and finally a sane default when neither is available (e.g.
// output redirected to a file with no COLUMNS in the environment).
func termWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	if c := os.Getenv("COLUMNS"); c != "" {
		if w, err := strconv.Atoi(c); err == nil && w > 0 {
			return w
		}
	}
	return 100
}
