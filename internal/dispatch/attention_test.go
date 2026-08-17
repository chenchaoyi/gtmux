package dispatch

import (
	"os"
	"testing"
)

// TestDispositionRoundTripAndLegacy confirms the disposition fields persist and
// that an entry written without them (legacy) still loads.
func TestDispositionRoundTripAndLegacy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	full := Task{
		ID: NewID(1000), Pane: "%1", Goal: "build", CreatedAt: 10,
		Disposition: "relayed", FirstSeen: 10, LastUpdate: 12,
	}
	if err := AddTask(full); err != nil {
		t.Fatal(err)
	}
	got, ok := LoadTask(full.ID)
	if !ok || got.Disposition != "relayed" || got.LastUpdate != 12 {
		t.Fatalf("disposition fields not round-tripped: %+v", got)
	}
	// A legacy entry (no disposition fields) still loads and defaults FirstSeen.
	legacy := Task{ID: NewID(2000), Pane: "%2", Goal: "test", CreatedAt: 20}
	if err := AddTask(legacy); err != nil {
		t.Fatal(err)
	}
	l, ok := LoadTask(legacy.ID)
	if !ok || l.Disposition != "" {
		t.Fatalf("legacy entry mangled: %+v", l)
	}
	if l.FirstSeen != 20 {
		t.Errorf("FirstSeen should default to CreatedAt, got %d", l.FirstSeen)
	}
}

// TestSetDispositionRecordsHow pins the free-text disposition write path that
// `gtmux tasks --resolve <id> [disposition]` rides.
func TestSetDispositionRecordsHow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	id := NewID(4000)
	_ = AddTask(Task{ID: id, Pane: "%4", Goal: "y", CreatedAt: 40})
	if !SetDisposition(id, "auto-answered", 400) {
		t.Fatal("set-disposition should succeed")
	}
	got, _ := LoadTask(id)
	if got.Disposition != "auto-answered" || got.LastUpdate != 400 {
		t.Fatalf("disposition not recorded: %+v", got)
	}
	// A missing task is a no-op.
	if SetDisposition("nope", "x", 400) {
		t.Error("setting a missing task's disposition should return false")
	}
}

// TestRetiredFieldsStillLoad pins the compatibility promise of
// slim-attention-ledger: an on-disk entry written when the ledger carried the
// tier/priority/surfaced/archive fields loads cleanly — unknown JSON fields
// are ignored, everything else survives.
func TestRetiredFieldsStillLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	id := NewID(7000)
	_ = AddTask(Task{ID: id, Pane: "%7", Goal: "old", CreatedAt: 70}) // creates the dir
	raw := `{"id":"` + id + `","pane":"%7","goal":"old","created_at":70,` +
		`"tier":"critical","priority":9,"surfaced":true,"surfaced_at":71,` +
		`"archived":false,"disposition":"relayed","first_seen":70}`
	if err := os.WriteFile(taskPath(id), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := LoadTask(id)
	if !ok {
		t.Fatal("an entry with retired fields must still load")
	}
	if got.Goal != "old" || got.Disposition != "relayed" || got.FirstSeen != 70 {
		t.Fatalf("living fields mangled while ignoring retired ones: %+v", got)
	}
}
