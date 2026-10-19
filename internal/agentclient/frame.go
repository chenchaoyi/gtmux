package agentclient

import (
	"github.com/chenchaoyi/gtmux/internal/ansi"
	"strings"
)

// dst chrome is normalized only on the current compact composer row. The
// shared draft guard still owns the decision; faint suggestions remain excluded.
func dstFrame(frame string, colored bool) string {
	original := strings.Split(frame, "\n")
	clean := frame
	if colored {
		clean = ansi.StripDroppingFaint(frame)
	} else {
		clean = ansi.Strip(frame)
	}
	lines := strings.Split(clean, "\n")
	for i := len(original) - 1; i >= 0; i-- {
		row := strings.TrimLeft(ansi.Strip(original[i]), "│╭╰╮╯─ \t")
		if !strings.HasPrefix(row, "⌸ ❯") {
			continue
		}
		line := lines[i]
		at := strings.Index(line, "❯")
		if at >= 0 {
			value := strings.TrimPrefix(line[at+len("❯"):], " ")
			value = strings.TrimPrefix(value, "INSERT ")
			value = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(value), "⛶"))
			lines[i] = "❯ " + value
		}
		break
	}
	return strings.Join(lines, "\n")
}
