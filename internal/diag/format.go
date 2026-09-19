package diag

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// Format renders one entry as the line `gtmux logs` prints: time, component, the level
// when it is not routine, the event, then who did what to what and how it ended for an
// action, or the message for a diagnostic, then the attributes in key order. color adds
// ANSI for warn and error; withDate prefixes the month and day for a window that spans
// days. docs/cli.md's example is rendered from here, not transcribed.
func Format(e Entry, color, withDate bool) string {
	t := e.Time()
	stamp := t.Format("15:04:05")
	if withDate {
		stamp = t.Format("01-02 15:04:05")
	}
	hue, reset := "", ""
	if color {
		reset = i18n.Reset
		switch e.Level {
		case "warn":
			hue = i18n.Amber
		case "error":
			hue = i18n.Red
		case "debug":
			hue = i18n.Dim
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %-7s ", stamp, e.Component)
	if e.Level == "warn" || e.Level == "error" {
		fmt.Fprintf(&b, "%s%-5s%s ", hue, e.Level, reset)
	}
	b.WriteString(e.Event + "  ")
	if e.Kind == KindAct {
		b.WriteString(e.Actor)
		if e.Target != "" {
			b.WriteString(" → " + e.Target)
		}
		b.WriteString(" " + hue + e.Outcome + reset)
		if e.Outcome != OK && e.Msg != "" {
			b.WriteString(" · " + e.Msg)
		}
	} else {
		b.WriteString(hue + e.Msg + reset)
	}
	if len(e.Attrs) > 0 {
		keys := make([]string, 0, len(e.Attrs))
		for k := range e.Attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString(" ·")
		for _, k := range keys {
			fmt.Fprintf(&b, " %s=%v", k, e.Attrs[k])
		}
	}
	return b.String()
}
