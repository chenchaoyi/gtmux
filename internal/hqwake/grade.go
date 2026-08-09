// The ATTENTION GRADE (hq-signal-ergonomics): a three-step scale every wake class and
// ledger entry projects onto, so a screen full of signal lines can be READ by weight
// instead of parsed word by word.
//
// The commander's report was 「屏幕上的信息有一些杂乱,最好能有一定的结构与颜色,让人能更好地
// 阅读与感知」(2026-08-07). The answer is deliberately a PROJECTION, not a new judgment:
// which class fires, and at what severity, is already decided elsewhere and hard-won. This
// file only says how loudly each of those already-made decisions should read.
package hqwake

// Grade is how loudly a signal should read. Ordered, so `>=` is a threshold test.
type Grade int

const (
	// GradeLedger — bookkeeping. Recorded, pullable, zero interrupt value.
	GradeLedger Grade = iota
	// GradeAttention — a line is blocked or has changed in a way worth knowing.
	GradeAttention
	// GradeDecision — needs the commander: irreversible, or carrying money/safety
	// consequence, or an explicit ask.
	GradeDecision
)

// Grade glyphs. Chosen against the same bar the `»` sigil and `│` separator had to
// clear: they must survive a POSIX/missing locale, a CJK-heavy font, and an agent's
// composer — the incident where tmux rendered `✳` as `_` under a locale-less launchd
// serve is why that bar exists at all.
//
// So: no emoji (they carry variation-selector and presentation baggage — see the mobile
// U+FE0E rule), and nothing outside the blocks the signal grammar already relies on.
// `·` is Latin-1 Supplement like `»`; `◆` and `▸` are Geometric Shapes like `│`'s Box
// Drawing neighbours. The ramp reads by SHAPE and weight, not by fill alone, so it still
// works in a monochrome terminal — colour is added on top where a surface owns it, never
// as the only carrier.
const (
	glyphDecision  = "◆"
	glyphAttention = "▸"
	glyphLedger    = "·"
)

// Glyph returns the grade's fixed-position marker.
func (g Grade) Glyph() string {
	switch g {
	case GradeDecision:
		return glyphDecision
	case GradeAttention:
		return glyphAttention
	default:
		return glyphLedger
	}
}

// Name is the grade's stable token (logs, tests, the docs table).
func (g Grade) Name() string {
	switch g {
	case GradeDecision:
		return "decision"
	case GradeAttention:
		return "attention"
	default:
		return "ledger"
	}
}

// classGrade projects each wake class onto the scale. It is DERIVED from the delivery
// priority the channel already assigns — the same fact, read for a different purpose —
// except where a class's interrupt value genuinely differs from its queue position:
//
//   - `tick` and `unread` queue at PriorityStanding so they never jump a decision, but a
//     tick is a brief and `unread` is a pull request: both are ledger-grade reading.
//   - `self-rotate` is standing in the queue and a DECISION on screen: it is the one knock
//     whose subject is HQ's own fitness to judge, and only HQ can act on it.
//   - the degradation alarms are decision-grade wherever they queue: perception itself is
//     broken, and no one but the commander can be told that.
var classGrade = map[string]Grade{
	ClassWaiting:      GradeDecision,
	ClassAsks:         GradeDecision,
	ClassGoalChanged:  GradeDecision,
	ClassCrash:        GradeDecision,
	ClassFeedDegraded: GradeDecision,
	ClassWakeDegraded: GradeDecision,
	ClassSelfRotate:   GradeDecision,

	ClassDone:         GradeAttention,
	ClassResolved:     GradeAttention,
	ClassNewSession:   GradeAttention,
	ClassReapSuggest:  GradeAttention,
	ClassStuckWaiting: GradeAttention,
	ClassResourceWarn: GradeAttention,
	ClassLimitsWarn:   GradeAttention,
	ClassUsageWarn:    GradeAttention,

	ClassTick:      GradeLedger,
	ClassUnread:    GradeLedger,
	ClassDistill:   GradeLedger,
	ClassSelfCheck: GradeLedger,
}

// GradeOf returns a class's grade, matching the same `waiting·permission` → `waiting`
// stem rule PriorityOf uses. An unknown class reads as attention: a signal gtmux chose to
// deliver is at least worth knowing, and silently grading it as bookkeeping would hide it.
func GradeOf(class string) Grade {
	if g, ok := classGrade[class]; ok {
		return g
	}
	if stem, _, found := cutClass(class); found {
		if g, ok := classGrade[stem]; ok {
			return g
		}
	}
	return GradeAttention
}

// GradeOfSeverity projects a STREAM record's severity onto the same scale the wake
// classes use, so an event read in `gtmux events` and the knock that announced it read
// alike. The severity tiers were built for exactly this question (what deserves
// interrupting whom), so the mapping is one-to-one rather than a fresh opinion.
func GradeOfSeverity(sev string) Grade {
	switch sev {
	case "important":
		return GradeDecision
	case "notable":
		return GradeAttention
	default: // routine, or a legacy record with no severity
		return GradeLedger
	}
}

// GradeOfTier projects an attention-LEDGER entry's surfacing tier onto the same scale,
// so a row in `gtmux tasks` reads at the weight the knock about it would have had. The
// tiers were defined by exactly this question (does this deserve the user's attention
// now), so like GradeOfSeverity this is a rename, not a second opinion. An entry with no
// tier — every entry written before the attention ledger existed — reads as attention:
// it is on a list of things awaiting a decision, which is not bookkeeping.
func GradeOfTier(tier string) Grade {
	switch tier {
	case "critical":
		return GradeDecision
	case "quiet":
		return GradeLedger
	default: // normal, or a legacy entry with no tier
		return GradeAttention
	}
}

// Color returns the grade's ANSI colour. It reuses the product's existing status
// palette rather than inventing a second one: decision is the red the radar already
// uses for "needs you", attention the cyan of work in flight, and ledger is dim —
// present, and asking for nothing. Callers gate on i18n.ColorEnabled(); the glyph
// carries the grade on its own, so a colourless terminal loses nothing.
func (g Grade) Color() string {
	switch g {
	case GradeDecision:
		return "\033[31m" // red
	case GradeAttention:
		return "\033[36m" // cyan
	default:
		return "\033[2m" // dim
	}
}

// Paint wraps s in the grade's colour when on; identity when off, so a non-tty or
// NO_COLOR caller emits exactly the bytes it emitted before this existed.
func (g Grade) Paint(s string, on bool) string {
	if !on || s == "" {
		return s
	}
	return g.Color() + s + "\033[0m"
}
