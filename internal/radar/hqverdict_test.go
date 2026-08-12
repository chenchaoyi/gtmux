package radar

import "testing"

func hqRow(status string) DigestRow {
	return DigestRow{Loc: "hq:0.0", Agent: "Claude Code", Role: "supervisor", Status: status}
}
func workerRow(name, status string, since int64) DigestRow {
	return DigestRow{Loc: name + ":0.0", Agent: "Claude Code", Status: status, Since: since}
}

// The priority order IS the contract — every surface reads it and none may re-derive it.
func TestHQVerdict_Priority(t *testing.T) {
	cases := []struct {
		name     string
		rows     []DigestRow
		critical bool
		want     string
	}{
		{"quiet fleet", []DigestRow{hqRow("idle"), workerRow("api", "idle", 0)}, false, VerdictNormal},
		{"hq working", []DigestRow{hqRow("working")}, false, VerdictWorking},
		{"a worker waiting", []DigestRow{hqRow("idle"), workerRow("api", "waiting", 5)}, false, VerdictNeedsYou},
		// The two the phone was missing entirely.
		{"HQ's own call outranks a quiet fleet", []DigestRow{hqRow("waiting"), workerRow("api", "idle", 0)}, false, VerdictHQCall},
		{"a red machine is never normal", []DigestRow{hqRow("idle"), workerRow("api", "idle", 0)}, true, VerdictResource},
		// ...and their order against each other.
		{"HQ's call outranks a waiting worker", []DigestRow{hqRow("waiting"), workerRow("api", "waiting", 5)}, false, VerdictHQCall},
		{"a waiting worker outranks a red machine", []DigestRow{hqRow("idle"), workerRow("api", "waiting", 5)}, true, VerdictNeedsYou},
		{"a red machine outranks HQ merely working", []DigestRow{hqRow("working")}, true, VerdictResource},
	}
	for _, c := range cases {
		v := hqVerdict(c.rows, c.critical)
		if v == nil {
			t.Fatalf("%s: no verdict", c.name)
		}
		if v.State != c.want {
			t.Errorf("%s: state = %q, want %q", c.name, v.State, c.want)
		}
	}
}

// No supervisor ⇒ no verdict. The row's ABSENCE is the "absent" state, which every
// surface already reads; inventing a sixth string here would give them two ways to spell
// the same thing.
func TestHQVerdict_NoSupervisor(t *testing.T) {
	if v := hqVerdict([]DigestRow{workerRow("api", "waiting", 5)}, true); v != nil {
		t.Fatalf("a fleet with no supervisor must carry no verdict, got %+v", v)
	}
}

// The facts a headline is built from. Getting `first` wrong is how a surface names the
// wrong session in "X needs you".
func TestHQVerdict_Facts(t *testing.T) {
	rows := []DigestRow{
		hqRow("idle"),
		workerRow("recent", "waiting", 900),
		workerRow("oldest", "waiting", 100), // waited longest → the one to unblock
		workerRow("calm", "idle", 0),
	}
	v := hqVerdict(rows, false)
	if v.Waiting != 2 {
		t.Errorf("waiting = %d, want 2", v.Waiting)
	}
	if v.Workers != 3 {
		t.Errorf("workers = %d, want 3 (the supervisor is never a worker)", v.Workers)
	}
	if v.First != "oldest" {
		t.Errorf("first = %q, want the longest-waiting session", v.First)
	}
}

// A native row carries no locator; the name must still come out as something printable.
func TestHQVerdict_NativeRowName(t *testing.T) {
	rows := []DigestRow{hqRow("idle"), {Agent: "Codex", Source: "native", Status: "waiting"}}
	if v := hqVerdict(rows, false); v.First != "Codex" {
		t.Errorf("first = %q, want the agent label when there is no locator", v.First)
	}
}
