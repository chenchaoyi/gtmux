package limits

import (
	"regexp"
	"strconv"
	"strings"
)

// The `claude -p "/usage"` print form emits one line per window, e.g.:
//
//	Current session: 11% used · resets Jul 13 at 1:30am (Asia/Shanghai)
//	Current week (all models): 58% used · resets Jul 17 at 10:59pm (Asia/Shanghai)
//	Current week (Fable): 88% used · resets Jul 17 at 10:59pm (Asia/Shanghai)
//
// We parse defensively: any line with "<label>: <n>% used" (optionally "· resets
// <when>") — so a future reorder/rename still yields windows, and the noisy
// "what's contributing" prose below is ignored (no "% used" match on those).
var lineRe = regexp.MustCompile(`^\s*(?:Current\s+)?(.+?):\s*(\d{1,3})%\s*used(?:\s*[·\-|]\s*resets?\s+(.+?))?\s*$`)

// parse extracts the windows from the command's stdout. Order is preserved.
func parse(out string) []Window {
	var wins []Window
	for _, raw := range strings.Split(out, "\n") {
		m := lineRe.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		pct, err := strconv.Atoi(m[2])
		if err != nil || pct < 0 || pct > 100 {
			continue
		}
		reset := strings.TrimSpace(m[3])
		// Drop a trailing "(Asia/Shanghai)"-style tz for compactness.
		if i := strings.LastIndex(reset, " ("); i > 0 {
			reset = reset[:i]
		}
		wins = append(wins, Window{
			Label:   normalizeLabel(m[1]),
			PctUsed: pct,
			ResetAt: reset,
		})
	}
	return wins
}

// normalizeLabel lowercases + trims the window label ("Current session" →
// "session", "week (all models)" kept) for stable keys/warn strings.
func normalizeLabel(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "current ")
	return strings.TrimSpace(s)
}
