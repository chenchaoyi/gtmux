package app

import (
	"os"
	"strconv"

	"golang.org/x/term"
)

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
