package hook

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
)

// The reap sweep runs in the hook on EVERY Stop, so what it reads is paid on every turn-end
// across the fleet. It read the whole journal once per dispatch that survived the cheap
// gates (about 110ms each on a real 16 MB journal), and a flagship the commander drives is
// "idle and done" in the ledger forever, so it survived them on every Stop, forever.

// fakeReapIO answers from a fixed journal and counts what the sweep did.
type fakeReapIO struct {
	recs     []events.Record
	reads    int
	declined map[string]bool
}

func (f *fakeReapIO) io() reapIO {
	return reapIO{
		idleSince:    func(string) int64 { return 1 }, // idle since forever
		decided:      func(id string) bool { return f.declined[id] },
		markDeclined: func(id string) { f.declined[id] = true },
		journal:      func() []events.Record { f.reads++; return f.recs },
	}
}

const reapNow = int64(100_000)

// before is the journal's history from before any of these dispatches.
var reapBefore = events.Record{Seq: 1, Ts: 10, Event: "SessionStart", Pane: "%1"}

func typed(seq, ts int64, pane, words string) events.Record {
	return events.Record{Seq: seq, Ts: ts, Event: "UserPromptSubmit", Pane: pane, Origin: events.OriginInstruction, Summary: words}
}

func delivered(seq, ts int64, pane, words string) []events.Record {
	return []events.Record{
		{Seq: seq, Ts: ts, Event: "UserPromptSubmit", Pane: pane, Summary: words},
		{Seq: seq + 1, Ts: ts, Event: events.AuditEventSend, Pane: pane, Actor: "hq", Summary: "landed: " + words},
	}
}

func TestTheSweepReadsTheJournalOnceHoweverManySurvive(t *testing.T) {
	recs := []events.Record{reapBefore}
	recs = append(recs, delivered(10, 200, "%31", "派给你的第一件事")...)
	recs = append(recs, delivered(20, 200, "%32", "派给你的第二件事")...)
	recs = append(recs, typed(30, 300, "%19", "发版吧"))
	f := &fakeReapIO{recs: recs, declined: map[string]bool{}}
	tasks := []dispatch.Task{
		{ID: "w1", Pane: "%31", CreatedAt: 100},
		{ID: "w2", Pane: "%32", CreatedAt: 100},
		{ID: "flagship", Pane: "%19", CreatedAt: 100},
	}
	got := reapCandidates(tasks, reapNow, 60, f.io())
	if f.reads != 1 {
		t.Errorf("three survivors read the journal %d times, want once", f.reads)
	}
	if len(got) != 2 || got[0].ID != "w1" || got[1].ID != "w2" {
		t.Errorf("candidates = %v, want the two pure workers", got)
	}
}

// Someone typing into a pane after its dispatch is an answer that cannot change, so it is
// written down, and the next sweep does not read the journal to find it out again.
func TestASettledAnswerIsNotWorkedOutAgain(t *testing.T) {
	recs := append([]events.Record{reapBefore}, typed(30, 300, "%19", "发版吧"))
	f := &fakeReapIO{recs: recs, declined: map[string]bool{}}
	tasks := []dispatch.Task{{ID: "flagship", Pane: "%19", CreatedAt: 100}}

	if got := reapCandidates(tasks, reapNow, 60, f.io()); len(got) != 0 {
		t.Fatalf("a driven session was offered for reclaiming: %v", got)
	}
	if !f.declined["flagship"] {
		t.Fatal("a session someone is typing into was not written down as settled")
	}
	f.reads = 0
	reapCandidates(tasks, reapNow, 60, f.io())
	if f.reads != 0 {
		t.Errorf("a settled session read the journal again (%d reads)", f.reads)
	}
}

// Not knowing YET is not settled. A dispatch that has not been given anything since must
// stay a question, or it could never be suggested once its work arrived.
func TestAnUnansweredQuestionIsNotWrittenDown(t *testing.T) {
	f := &fakeReapIO{recs: []events.Record{reapBefore}, declined: map[string]bool{}}
	tasks := []dispatch.Task{{ID: "fresh", Pane: "%40", CreatedAt: 100}}
	if got := reapCandidates(tasks, reapNow, 60, f.io()); len(got) != 0 {
		t.Errorf("a pane with no prompts was offered: %v", got)
	}
	if f.declined["fresh"] {
		t.Error("\"no prompts yet\" was written down as a permanent answer")
	}
}

// And the journal is not read at all when nothing survives the cheap gates.
func TestNoSurvivorNoRead(t *testing.T) {
	f := &fakeReapIO{recs: []events.Record{reapBefore}, declined: map[string]bool{"x": true}}
	reapCandidates([]dispatch.Task{{ID: "x", Pane: "%1", CreatedAt: 100}, {ID: "nopane"}}, reapNow, 60, f.io())
	if f.reads != 0 {
		t.Errorf("the journal was read %d times with nothing to judge", f.reads)
	}
}
