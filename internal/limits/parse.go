package limits

import (
	"regexp"
	"strconv"
	"strings"
	"time"
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

// parse extracts the windows from the command's stdout. Order is preserved. `now`
// anchors the year of a reset time the agent prints without one.
func parse(out string, now time.Time) []Window {
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
		// The trailing "(Asia/Shanghai)" is dropped from the human string for
		// compactness, and kept as the zone the epoch is computed in.
		tz := ""
		if i := strings.LastIndex(reset, " ("); i > 0 && strings.HasSuffix(reset, ")") {
			tz = reset[i+2 : len(reset)-1]
			reset = reset[:i]
		}
		kind, model := classify(m[1])
		wins = append(wins, Window{
			Label:     normalizeLabel(m[1]),
			PctUsed:   pct,
			ResetAt:   reset,
			ResetUnix: resetUnix(reset, tz, now),
			Kind:      kind,
			Model:     model,
		})
	}
	return wins
}

var weekModelRe = regexp.MustCompile(`(?i)^week \((.+)\)$`)

// classify reads a printed label into a Kind (+ Model for a per-model week). The
// model keeps the agent's own capitalisation ("Fable"); the label is lowercased
// separately for stable keys.
func classify(label string) (kind, model string) {
	l := normalizeLabel(label)
	switch l {
	case "session":
		return KindSession, ""
	case "week":
		return KindWeek, ""
	case "week (all models)":
		return KindWeekAll, ""
	}
	orig := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(label), "Current "), "current "))
	if m := weekModelRe.FindStringSubmatch(orig); m != nil {
		return KindWeekModel, strings.TrimSpace(m[1])
	}
	return "", ""
}

// resetLayouts are the shapes Claude prints a reset in ("Jul 13 at 1:30am", "Jul 13 at 1am").
var resetLayouts = []string{"Jan 2 at 3:04pm", "Jan 2 at 3pm"}

// resetUnix turns the printed reset into an epoch, in the zone the line named (else the
// machine's), with the year taken from `now`: a reset is at most a week ahead, so a
// parsed time more than a day in the past belongs to next year (a January reset read in
// December). 0 when the string has no known shape — the human string still shows.
func resetUnix(reset, tz string, now time.Time) int64 {
	if reset == "" {
		return 0
	}
	loc := now.Location()
	if tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	for _, layout := range resetLayouts {
		t, err := time.ParseInLocation(layout, reset, loc)
		if err != nil {
			continue
		}
		y := now.In(loc).Year()
		t = time.Date(y, t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, loc)
		if t.Before(now.Add(-24 * time.Hour)) {
			t = t.AddDate(1, 0, 0)
		}
		return t.Unix()
	}
	return 0
}

// normalizeLabel lowercases + trims the window label ("Current session" →
// "session", "week (all models)" kept) for stable keys/warn strings.
func normalizeLabel(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "current ")
	return strings.TrimSpace(s)
}
