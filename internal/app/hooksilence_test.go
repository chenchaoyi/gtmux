package app

import (
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// silentPane is the judgement this row makes, isolated from tmux and the event log so
// the RULE can be tested as a rule: a pane that is painting while its agent has said
// nothing for a long time is a broken channel. Neither half means anything alone — a
// quiet pane may simply be idle, and a quiet event stream may mean nobody is working.
func silentPane(paneActivity, lastEvent, now int64) bool {
	if paneActivity == 0 || now-paneActivity > hookSilenceGrace {
		return false
	}
	return now-lastEvent > hookSilenceGrace
}

// The case that went unseen for six hours: Codex re-read a hooks file that had been
// rewritten under it, stopped firing, and an approval sat unanswered while the radar
// showed the session working. Its hooks were installed AND complete the whole time —
// which is why the other two hook rows could not see it.
func TestABusyPaneWithASilentAgentIsReported(t *testing.T) {
	const now = 1_800_000_000
	if !silentPane(now-30, now-6*3600, now) {
		t.Error("a pane painting now whose agent last spoke six hours ago must be reported")
	}
}

// A quiet pane is not evidence of anything. Most panes are idle most of the time, and a
// row that fires on them would be noise nobody reads.
func TestAQuietPaneIsNotReported(t *testing.T) {
	const now = 1_800_000_000
	if silentPane(now-5*3600, now-6*3600, now) {
		t.Error("an idle pane with an idle agent is the normal state, not a fault")
	}
}

// A long turn is not a fault either: an agent can work inside one turn for a long
// stretch without an event, which is why the grace is generous.
func TestARecentEventIsEnough(t *testing.T) {
	const now = 1_800_000_000
	if silentPane(now-30, now-hookSilenceGrace+60, now) {
		t.Error("an agent that spoke inside the grace must not be reported")
	}
	if !silentPane(now-30, now-hookSilenceGrace-60, now) {
		t.Error("past the grace it must be")
	}
}

// The row reads the event log for the newest event per pane; make sure that read works
// against a real (temp) log rather than only in theory.
func TestEventsReadGivesThePaneItsNewest(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	events.Append(events.Record{Ts: now - 100, Event: "Stop", Pane: "%1"})
	events.Append(events.Record{Ts: now - 10, Event: "UserPromptSubmit", Pane: "%1"})
	events.Append(events.Record{Ts: now - 50, Event: "Stop", Pane: "%2"})

	last := map[string]int64{}
	for _, r := range events.Read(now-3600, now) {
		if r.Pane != "" && r.Ts > last[r.Pane] {
			last[r.Pane] = r.Ts
		}
	}
	if last["%1"] != now-10 {
		t.Errorf("%%1 newest = %d, want %d", last["%1"], now-10)
	}
	if last["%2"] != now-50 {
		t.Errorf("%%2 newest = %d, want %d", last["%2"], now-50)
	}
	_ = state.Dir()
}
