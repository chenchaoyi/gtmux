package dispatch

import "testing"

// TestAttentionFieldsRoundTripAndLegacy confirms the additive attention fields
// persist and that an entry written without them (legacy) still loads.
func TestAttentionFieldsRoundTripAndLegacy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	full := Task{
		ID: NewID(1000), Pane: "%1", Goal: "build", CreatedAt: 10,
		Tier: "critical", Priority: 5, Surfaced: true, SurfacedAt: 12,
		Disposition: "relayed", FirstSeen: 10, LastUpdate: 12,
	}
	if err := AddTask(full); err != nil {
		t.Fatal(err)
	}
	got, ok := LoadTask(full.ID)
	if !ok || got.Tier != "critical" || got.Priority != 5 || !got.Surfaced || got.Disposition != "relayed" {
		t.Fatalf("attention fields not round-tripped: %+v", got)
	}
	// A legacy entry (no attention fields) still loads and defaults FirstSeen.
	legacy := Task{ID: NewID(2000), Pane: "%2", Goal: "test", CreatedAt: 20}
	if err := AddTask(legacy); err != nil {
		t.Fatal(err)
	}
	l, ok := LoadTask(legacy.ID)
	if !ok || l.Tier != "" || l.Priority != 0 {
		t.Fatalf("legacy entry mangled: %+v", l)
	}
	if l.FirstSeen != 20 {
		t.Errorf("FirstSeen should default to CreatedAt, got %d", l.FirstSeen)
	}
}

func TestPromoteIsInPlaceNoDuplicate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	id := NewID(3000)
	_ = AddTask(Task{ID: id, Pane: "%3", Goal: "x", CreatedAt: 30, Tier: "quiet"})
	if !Promote(id, "critical", 9, 100) {
		t.Fatal("promote should succeed")
	}
	// Same entry mutated — no duplicate created.
	if n := len(ListTasks()); n != 1 {
		t.Fatalf("promote created a duplicate: %d entries", n)
	}
	got, _ := LoadTask(id)
	if got.Tier != "critical" || got.Priority != 9 || got.LastUpdate != 100 {
		t.Fatalf("promote didn't apply: %+v", got)
	}
	// A missing task is a no-op.
	if Promote("nope", "critical", 1, 100) {
		t.Error("promoting a missing task should return false")
	}
}

func TestMarkSurfacedAndDisposition(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	id := NewID(4000)
	_ = AddTask(Task{ID: id, Pane: "%4", Goal: "y", CreatedAt: 40})
	if !MarkSurfaced(id, 200) {
		t.Fatal("mark surfaced should succeed")
	}
	got, _ := LoadTask(id)
	if !got.Surfaced || got.SurfacedAt != 200 {
		t.Fatalf("surfaced not recorded: %+v", got)
	}
	// SurfacedAt is stamped once (a re-mark doesn't move it).
	_ = MarkSurfaced(id, 300)
	got, _ = LoadTask(id)
	if got.SurfacedAt != 200 {
		t.Errorf("SurfacedAt should be stamped once, got %d", got.SurfacedAt)
	}
	_ = SetDisposition(id, "auto-answered", 400)
	got, _ = LoadTask(id)
	if got.Disposition != "auto-answered" || got.LastUpdate != 400 {
		t.Fatalf("disposition not recorded: %+v", got)
	}
}

func TestArchiveMovesOutOfLiveSet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a := NewID(5000)
	b := NewID(6000)
	_ = AddTask(Task{ID: a, Pane: "%5", Goal: "done-thing", CreatedAt: 50})
	_ = AddTask(Task{ID: b, Pane: "%6", Goal: "live-thing", CreatedAt: 60})

	if !ArchiveTask(a, 500) {
		t.Fatal("archive should succeed")
	}
	// Live set no longer has a; b remains.
	live := ListTasks()
	if len(live) != 1 || live[0].ID != b {
		t.Fatalf("live set after archive = %+v", live)
	}
	// a is retro-queryable via the archive.
	arch := ListArchived()
	if len(arch) != 1 || arch[0].ID != a || !arch[0].Archived || arch[0].ArchivedAt != 500 {
		t.Fatalf("archived set = %+v", arch)
	}
	// LoadTask (live) misses it; LoadAnyTask finds it.
	if _, ok := LoadTask(a); ok {
		t.Error("archived task should not load from the live set")
	}
	if got, ok := LoadAnyTask(a); !ok || got.ID != a {
		t.Fatalf("LoadAnyTask should find the archived entry: %+v ok=%v", got, ok)
	}
	// Archiving a missing task is a no-op.
	if ArchiveTask("nope", 500) {
		t.Error("archiving a missing task should return false")
	}
}
