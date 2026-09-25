package advice

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func asHQ(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.HQHome(), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(state.HQHome())
}

func TestAdviceRoundTrip(t *testing.T) {
	asHQ(t)
	id, err := Give("hold the release until nginx takes a 12MB body", "measured 413 through the tunnel", "v1.0.49", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if id != "a1" {
		t.Errorf("first id = %q, want a1", id)
	}
	id2, _ := Give("second one", "", "", 1100)
	if id2 != "a2" {
		t.Errorf("second id = %q, want a2", id2)
	}
	if err := Mark("a1", Taken, "对，先修", 1200); err != nil {
		t.Fatal(err)
	}
	live, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 2 {
		t.Fatalf("live = %d, want 2", len(live))
	}
	if live[0].Outcome != Taken || live[0].Words != "对，先修" || live[0].MarkAt != 1200 {
		t.Errorf("a1 folded wrong: %+v", live[0])
	}
	if !live[1].Open() {
		t.Errorf("a2 should still be open: %+v", live[1])
	}
}

// The commander is allowed to change their mind, and the reversal must not erase that it
// happened: the log keeps both records, the fold shows the last one.
func TestTheLastMarkWinsAndTheLogKeepsBoth(t *testing.T) {
	asHQ(t)
	id, _ := Give("do it this way", "", "", 1000)
	if err := Mark(id, Declined, "先不动", 1100); err != nil {
		t.Fatal(err)
	}
	if err := Mark(id, Taken, "想了想还是按你说的", 1200); err != nil {
		t.Fatal(err)
	}
	live, _ := List()
	if live[0].Outcome != Taken {
		t.Errorf("outcome = %q, want the later one (%s)", live[0].Outcome, Taken)
	}
	b, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(b), `"op":"mark"`); n != 2 {
		t.Errorf("the ledger holds %d marks, want both kept as history", n)
	}
}

func TestMarkRefusesWhatItCannotRecord(t *testing.T) {
	asHQ(t)
	id, _ := Give("something", "", "", 1000)
	if err := Mark("a99", Taken, "", 1100); err == nil {
		t.Error("marking an id that does not exist was accepted")
	}
	if err := Mark(id, "maybe", "", 1100); err == nil {
		t.Error("an unknown outcome was accepted")
	}
	if _, err := Give("   ", "", "", 1000); err == nil {
		t.Error("empty advice was accepted")
	}
	if _, err := Give(strings.Repeat("x", whatMax+1), "", "", 1000); err == nil {
		t.Error("advice over the limit was accepted instead of refused")
	}
}

// The tally is the point of the ledger, so what it counts and what it leaves out is pinned.
func TestTallyCountsAnswersNotSilence(t *testing.T) {
	live := []Advice{
		{ID: "a1", At: 100, Outcome: Taken},
		{ID: "a2", At: 200, Outcome: Declined},
		{ID: "a3", At: 300, Outcome: Moot},
		{ID: "a4", At: 400},
		{ID: "a5", At: 50, Outcome: Taken}, // before the window
	}
	t2 := Count(live, 100)
	if t2.Given != 4 || t2.Taken != 1 || t2.Declined != 1 || t2.Moot != 1 || t2.Open != 1 {
		t.Fatalf("tally = %+v", t2)
	}
	rate, ok := t2.TakenRate()
	if !ok || rate != 0.5 {
		t.Errorf("rate = %v (ok=%v), want 0.5: open and overtaken advice are not verdicts", rate, ok)
	}
	// Nothing answered yet: there is no rate to report, and reporting 0% would read as
	// a judgement that has never landed.
	if _, ok := Count([]Advice{{ID: "a1", At: 1}}, 0).TakenRate(); ok {
		t.Error("a rate was reported with nothing answered")
	}
}

func TestParseSince(t *testing.T) {
	const now = 1_000_000
	for _, tc := range []struct {
		in   string
		want int64
	}{
		{"", 0}, {"all", 0}, {"30d", now - 30*86400}, {"12h", now - 12*3600},
	} {
		got, err := ParseSince(tc.in, now)
		if err != nil || got != tc.want {
			t.Errorf("ParseSince(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}
	for _, bad := range []string{"30", "d30", "0d", "-5d", "30w"} {
		if _, err := ParseSince(bad, now); err == nil {
			t.Errorf("ParseSince(%q) was accepted", bad)
		}
	}
}

// A torn line must not take the rest of the ledger with it.
func TestATornLineDoesNotLoseTheLedger(t *testing.T) {
	asHQ(t)
	if _, err := Give("first", "", "", 1000); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(Path(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("{\"v\":1,\"op\":\"give\",\"id\":\"a2\"\n") // cut off mid-write
	f.Close()
	if _, err := Give("third", "", "", 1200); err != nil {
		t.Fatal(err)
	}
	live, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 2 {
		t.Fatalf("live = %d, want the two whole records", len(live))
	}
}

func TestTheLedgerLivesInHQsHome(t *testing.T) {
	asHQ(t)
	if got, want := Path(), filepath.Join(state.HQHome(), "advice.jsonl"); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}
