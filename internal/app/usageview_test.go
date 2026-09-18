package app

import (
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/limits"
	"github.com/chenchaoyi/gtmux/internal/radar"
	uwatch "github.com/chenchaoyi/gtmux/internal/usage"
)

// The bar is twelve cells on both screens. They are read one after the other, and two
// lengths for the same window reads as two different measurements.
func TestThePlanBarIsTheSameOnBothScreens(t *testing.T) {
	for _, c := range []struct{ pct, filled int }{{0, 0}, {50, 6}, {99, 11}, {100, 12}, {140, 12}, {-3, 0}} {
		got := strings.Count(stripANSI(planBar(c.pct, "")), "█")
		if got != c.filled {
			t.Errorf("planBar(%d) filled %d cells, want %d", c.pct, got, c.filled)
		}
		if n := len([]rune(stripANSI(planBar(c.pct, "")))); n != 12 {
			t.Errorf("planBar(%d) is %d cells wide, want 12", c.pct, n)
		}
	}
}

// A session burning nothing writes "0", not "0/m": the unit spends three columns saying
// that nothing is happening.
func TestARestingSessionDropsTheRateUnit(t *testing.T) {
	if got := rateOf(0); got != "0" {
		t.Errorf("rateOf(0) = %q, want %q", got, "0")
	}
	if got := rateOf(884); !strings.HasSuffix(got, "/m") {
		t.Errorf("rateOf(884) = %q, want a per-minute unit", got)
	}
}

// The closing line answers what the rows raise and do not: when the full window comes
// back, and what still works until then. It used to restate the percentage that was
// already on the row above it.
func TestTheCapLineSaysWhenItComesBackAndWhatStillWorks(t *testing.T) {
	defer i18n.SetLang("en")
	i18n.SetLang("en")
	now := time.Unix(1_700_000_000, 0)
	rep := limits.Report{Windows: []limits.Window{
		{Label: "claude week (fable)", PctUsed: 100, Tier: limits.TierFull, ResetAt: "Sep 18 at 11pm"},
		{Label: "claude week (all models)", PctUsed: 74, Kind: limits.KindWeekAll, AgentName: "Claude Code"},
	}}
	got := stripANSI(capLine(rep, now))
	for _, want := range []string{"spent", "Sep 18 at 11pm", "Claude Code", "74%"} {
		if !strings.Contains(got, want) {
			t.Errorf("cap line %q does not say %q", got, want)
		}
	}
	// Nothing at its cap: no sentence at all rather than a cheerful one.
	quiet := limits.Report{Windows: []limits.Window{{Label: "claude session", PctUsed: 22}}}
	if got := capLine(quiet, now); got != "" {
		t.Errorf("a healthy plan printed %q, want silence", got)
	}
}

// The screen leads with the plan, because that is the one number local counting cannot
// produce, and it says which period each column covers: one conversation's own total can
// exceed the week's, and a reader meeting both with no explanation concludes one is wrong.
//
// It also keeps the two meanings of "session" apart. A tmux session is what `restore` and
// `overview` count; what this screen lists is each agent CONVERSATION, and Claude's own
// five-hour window is named by its length. All three used to be called a session, twice
// on this one screen.
func TestTheUsageScreenLeadsWithThePlanAndNamesThePeriod(t *testing.T) {
	defer i18n.SetLang("en")
	i18n.SetLang("en")
	out := captureStdout(t, func() {
		printUsage(usageFixture(), time.Unix(1_700_000_000, 0))
	})
	plan, convos := strings.Index(out, "PLAN"), strings.Index(out, "CONVERSATIONS")
	if plan < 0 || convos < 0 {
		t.Fatalf("screen has no PLAN/CONVERSATIONS sections:\n%s", out)
	}
	if plan > convos {
		t.Error("the conversation list comes before the plan; the plan is the number local counting cannot produce")
	}
	if !strings.Contains(out, "since it started") {
		t.Error("the conversation column never says which period it covers")
	}
	if strings.Contains(out, "SESSIONS") {
		t.Error("the screen still calls agent conversations sessions, which is what tmux calls its own")
	}
	if strings.Count(out, "out ·") > 0 {
		t.Error("the column words are still repeated per row")
	}
}

// usageFixture is one fleet in the shape the screen renders: a working session, a warning
// one, and enough idle rows that the list has a tail to fold.
func usageFixture() radar.UsageReport {
	rows := []radar.UsageRow{
		{Loc: "hq:0.0", Agent: "Claude Code", Status: "working", Tok: 19_800_000, Ctx: 0.73, Rate: 884},
		{Loc: "web:0.0", Agent: "Claude Code", Status: "working", Tok: 291_000, Ctx: 0.32, Rate: 3100},
		{Loc: "app:0.0", Agent: "Claude Code", Status: "idle", Tok: 136_000, Ctx: 0.95, UsageWarn: "ctx 95%"},
	}
	for i := 0; i < 5; i++ {
		rows = append(rows, radar.UsageRow{Loc: "quiet:0.0", Agent: "Codex", Status: "idle", Tok: 10_000})
	}
	return radar.UsageReport{
		Sessions: rows,
		Limits: limits.Report{Windows: []limits.Window{
			{Label: "claude session", PctUsed: 16},
			{Label: "claude week (all models)", PctUsed: 74, Kind: limits.KindWeekAll},
		}},
		History: uwatch.History{TodayOut: 2_800_000, WeekOut: 16_200_000},
	}
}
