package events

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// tinyCap points HOME at a temp dir and sets a tiny eventsCapMB so rotation is
// reachable in a test without writing 20 MB. Returns the cap in bytes.
func tinyCap(t *testing.T, mb int) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	// state.Dir() = HOME/.local/share/gtmux
	if err := os.MkdirAll(filepath.Join(home, ".config", "gtmux"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "gtmux", "config.json"),
		[]byte(`{"eventsCapMB":`+itoaTest(mb)+`}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAppendAndRead(t *testing.T) {
	tinyCap(t, 20)
	now := time.Now().Unix()
	Append(Record{Ts: now - 100, Event: "UserPromptSubmit", State: "working", Loc: "a:0.0", Agent: "Claude Code"})
	Append(Record{Ts: now - 10, Event: "Waiting", State: "waiting", Kind: "permission", Loc: "b:0.0"})

	all := Read(0, now)
	if len(all) != 2 {
		t.Fatalf("read all = %d, want 2", len(all))
	}
	if all[1].Kind != "permission" || all[1].State != "waiting" {
		t.Errorf("record 2 = %+v", all[1])
	}
	// --since window drops the old one
	recent := Read(60, now)
	if len(recent) != 1 || recent[0].Loc != "b:0.0" {
		t.Errorf("since 60s = %+v", recent)
	}
}

func TestRotation(t *testing.T) {
	// cap 0 disables; use a 1-MB cap and write >1 MB to force a rotation.
	tinyCap(t, 1)
	now := time.Now().Unix()
	big := Record{Ts: now, Event: "Stop", State: "idle", Loc: "x:0.0", Agent: "Claude Code",
		Session: string(make([]byte, 400))} // ~fat line
	for i := 0; i < 4000; i++ { // ~4000 × ~450B ≈ 1.8 MB → crosses the 1 MB cap
		Append(big)
	}
	// The active file must be under the cap, and the rotated generation exists.
	fi, err := os.Stat(Path())
	if err != nil || fi.Size() >= (1<<20) {
		t.Fatalf("active file not rotated under cap: size=%d err=%v", fi.Size(), err)
	}
	if _, err := os.Stat(rotatedPath()); err != nil {
		t.Fatalf("rotated generation missing: %v", err)
	}
	// Read spans both generations (recent window still returns records).
	if len(Read(0, now)) == 0 {
		t.Error("Read across generations returned nothing")
	}
}

func TestCapZeroDisables(t *testing.T) {
	tinyCap(t, 0)
	Append(Record{Ts: 1, Event: "Stop", State: "idle"})
	if _, err := os.Stat(Path()); err == nil {
		t.Error("cap 0 should disable the log (no file written)")
	}
}

func TestFollowStreamsNewAndSurvivesRotation(t *testing.T) {
	tinyCap(t, 1)
	now := time.Now().Unix()
	Append(Record{Ts: now, Event: "old", State: "working", Loc: "seed"}) // pre-existing tail (skipped by follow)

	got := make(chan Record, 100)
	stop := make(chan struct{})
	go Follow(0, now, func(r Record) { got <- r }, stop)
	time.Sleep(300 * time.Millisecond) // let it open + seek to end

	Append(Record{Ts: now, Event: "new1", State: "working", Loc: "live1"})
	select {
	case r := <-got:
		if r.Event != "new1" {
			t.Fatalf("first followed = %q, want new1", r.Event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("follow did not emit a new append")
	}

	// Force a rotation, then append — the follower must re-open and keep emitting.
	_ = os.Rename(Path(), rotatedPath())
	Append(Record{Ts: now, Event: "afterRotate", State: "idle", Loc: "live2"})
	select {
	case r := <-got:
		if r.Event != "afterRotate" {
			t.Fatalf("post-rotation followed = %q, want afterRotate", r.Event)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("follow did not survive rotation")
	}
	close(stop)
}

// Severity is a deterministic tier over lifecycle records: a wait and an "asking"
// turn-end are important; a "report" turn-end and session lifecycle are notable;
// prompt submissions and ticks are routine.
func TestSeverity(t *testing.T) {
	cases := []struct {
		name string
		r    Record
		want string
	}{
		{"waiting-needs-user", Record{Event: "Waiting", Kind: "permission"}, SevImportant},
		{"asking-turn-end", Record{Event: "Stop", Class: "asking"}, SevImportant},
		{"report-turn-end", Record{Event: "Stop", Class: "report"}, SevNotable},
		{"stop-no-class", Record{Event: "Stop"}, SevNotable},
		{"session-start", Record{Event: "SessionStart"}, SevNotable},
		{"precompact", Record{Event: "PreCompact"}, SevNotable},
		{"prompt-submit", Record{Event: "UserPromptSubmit"}, SevRoutine},
		{"notification", Record{Event: "Notification"}, SevRoutine},
		{"waiting-no-kind", Record{Event: "Waiting"}, SevNotable},
	}
	for _, c := range cases {
		if got := Severity(c.r); got != c.want {
			t.Errorf("%s: Severity = %q, want %q", c.name, got, c.want)
		}
	}
}

// Rank orders the tiers for "this level and above"; an empty/legacy value ranks routine.
func TestSeverityRank(t *testing.T) {
	if !(SeverityRank(SevRoutine) < SeverityRank(SevNotable) &&
		SeverityRank(SevNotable) < SeverityRank(SevImportant)) {
		t.Error("severity ranks must be routine < notable < important")
	}
	if SeverityRank("") != 0 || SeverityRank("bogus") != 0 {
		t.Error("an empty/unknown severity must rank as routine (0)")
	}
}

// Append stamps the tier at the source and round-trips it; an explicit value is kept.
func TestAppendStampsSeverity(t *testing.T) {
	tinyCap(t, 20)
	now := time.Now().Unix()
	Append(Record{Ts: now, Event: "Waiting", State: "waiting", Kind: "plan", Loc: "a:0.0"})
	Append(Record{Ts: now, Event: "UserPromptSubmit", State: "working", Loc: "b:0.0"})
	Append(Record{Ts: now, Event: "Stop", State: "idle", Loc: "c:0.0", Severity: SevImportant}) // explicit kept

	all := Read(0, now)
	if len(all) != 3 {
		t.Fatalf("read = %d, want 3", len(all))
	}
	if all[0].Severity != SevImportant {
		t.Errorf("waiting stamped %q, want important", all[0].Severity)
	}
	if all[1].Severity != SevRoutine {
		t.Errorf("prompt stamped %q, want routine", all[1].Severity)
	}
	if all[2].Severity != SevImportant {
		t.Errorf("explicit severity clobbered: %q", all[2].Severity)
	}
}

func TestParseHelpers(t *testing.T) {
	if clock(0) != "--:--:--" {
		t.Error("clock(0)")
	}
	if pad("ab", 4) != "ab  " {
		t.Errorf("pad = %q", pad("ab", 4))
	}
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
