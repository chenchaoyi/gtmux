package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPurposePreviewPreservesSources(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	read := func() []PurposePlacement {
		t.Helper()
		status, err := ReadStorageLayout()
		if err != nil {
			t.Fatal(err)
		}
		rows, err := InspectPurposePlacement(status)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	if rows := read(); len(rows) != 0 {
		t.Fatal("fresh installation has retained stores", rows)
	}
	if _, err := os.Stat(Dir()); !os.IsNotExist(err) {
		t.Fatal("inspection wrote storage")
	}
	before := layoutFixture(t, "events.jsonl", "original")
	want := func(from string, conflict bool) {
		t.Helper()
		for _, row := range read() {
			if row.Current == from && row.Destination == "diagnostics/events/events.jsonl" && row.Conflict == conflict {
				return
			}
		}
		t.Fatal("missing placement", from, conflict)
	}
	want("events.jsonl", false)
	if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	if err := ChangeWorkflowLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	from := "layout-v2/events/events.jsonl"
	want(from, false)
	other := layoutFixture(t, "diagnostics/events/events.jsonl", "different")
	want(from, true)
	wantLayoutContents(t, filepath.Join(Dir(), from), "original", before)
	wantLayoutContents(t, filepath.Join(Dir(), "diagnostics/events/events.jsonl"), "different", other)
}

func TestPurposeMoveRecoveryAndInverseInventory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	before := layoutFixture(t, "events.jsonl", "existing event")
	layoutFixture(t, "hq-feed/record", "kept")
	if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	if err := ChangeWorkflowLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"logs", "uploads", "terminal-input"} {
		if err := MkdirAll(filepath.Join(Dir(), key), PrivateDir); err != nil {
			t.Fatal(err)
		}
	}
	prior, err := readWorkflowJournal()
	if err != nil {
		t.Fatal(err)
	}
	j, err := preparePurposeJournal(prior, true)
	if err != nil {
		t.Fatal(err)
	}
	moves := j.Entries
	if len(moves) < 2 {
		t.Fatalf("forward plan: %v %v", moves, err)
	}
	if err := advancePurposeMove(moves[0], true); err != nil {
		t.Fatal(err)
	}
	if CheckStorageLayout() == nil {
		t.Fatal("interrupted move admitted a writer")
	}
	j, err = readWorkflowJournal()
	if err != nil || j.Version != purposeVersion {
		t.Fatal("lost pending journal", err)
	}
	if err := verifyPurposeMoves(moves, true, false); err != nil {
		t.Fatal(err)
	}
	if err := verifyPurposeMoves(moves, true, true); err == nil {
		t.Fatal("partial move reported complete")
	}
	block := moves[1]
	_, target, err := purposeEndpoints(block.Name, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := WritePrivateFile(target, []byte("keep both")); err != nil {
		t.Fatal(err)
	}
	if err := verifyPurposeMoves(moves, true, false); err == nil {
		t.Fatal("conflict accepted")
	}
	if err := advancePurposeMove(block, true); err == nil {
		t.Fatal("conflict overwritten")
	}
	if b, err := os.ReadFile(target); err != nil || string(b) != "keep both" {
		t.Fatal("overwritten target", err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := ChangePurposeLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	wantLayoutContents(t, StorePath("events.jsonl"), "existing event", before)
	newFile := layoutFixture(t, "diagnostics/events/events.seq", "created after migration")
	if err := ChangePurposeLayout(false, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	wantLayoutContents(t, StorePath("events.seq"), "created after migration", newFile)
	wantLayoutContents(t, StorePath("events.jsonl"), "existing event", before)
}

func TestPurposeHistoricalAttachmentReference(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old, canonical, err := referencePaths("uploads")
	if err != nil {
		t.Fatal(err)
	}
	file := "old attachment (一).txt"
	info := layoutFixture(t, filepath.Join("uploads", file), "historic")
	id, _, err := workflowPathObject(old)
	if err != nil {
		t.Fatal(err)
	}
	id.Name = "uploads"
	if err := advancePurposeMove(id, true); err != nil {
		t.Fatal(err)
	}
	ref, err := publishPurposeReference("uploads", id)
	if err != nil {
		t.Fatal(err)
	}
	wantLayoutContents(t, filepath.Join(old, file), "historic", info)
	if again, err := publishPurposeReference("uploads", id); err != nil || again != ref {
		t.Fatal("non-idempotent alias", err)
	}
	// Every externally issued path uses this stable spelling while the newer
	// private writer publishes to the physical category.
	fresh := layoutFixture(t, "attachments/uploads/new.txt", "new")
	wantLayoutContents(t, filepath.Join(old, "new.txt"), "new", fresh)
	saved := old + ".saved"
	if err := os.Rename(old, saved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("outside", old); err != nil {
		t.Fatal(err)
	}
	if ref.retire() == nil {
		t.Fatal("changed reference removed")
	}
	if got, _ := os.Readlink(old); got != "outside" {
		t.Fatal("unrelated link lost")
	}
	if err := os.Remove(old); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(saved, old); err != nil {
		t.Fatal(err)
	}
	if err := ref.retire(); err != nil {
		t.Fatal(err)
	}
	if err := advancePurposeMove(id, false); err != nil {
		t.Fatal(err)
	}
	wantLayoutContents(t, filepath.Join(old, file), "historic", info)
	wantLayoutContents(t, filepath.Join(old, "new.txt"), "new", fresh)
	if _, err := os.Lstat(canonical); !os.IsNotExist(err) {
		t.Fatal("second uploads store remains")
	}
}

func TestPurposeOfflineSwitchAndRollback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	original := layoutFixture(t, "uploads/history.png", "attachment")
	layoutFixture(t, "terminal-input/outcome.uncertain", "keep")
	if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	if err := ChangeWorkflowLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	if err := ChangePurposeLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	if err := ChangePurposeLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{layoutDir, workflowLayoutDir} {
		if _, err := os.Lstat(filepath.Join(Dir(), name)); !os.IsNotExist(err) {
			t.Fatalf("empty version directory remains: %s %v", name, err)
		}
	}
	if RootStorePath("uploads") != filepath.Join(Dir(), "attachments", "uploads") {
		t.Fatal("writer routed to old store")
	}
	wantLayoutContents(t, UploadReference("history.png"), "attachment", original)
	newer := layoutFixture(t, "attachments/uploads/new.png", "new")
	if err := ChangePurposeLayout(false, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	wantLayoutContents(t, UploadReference("new.png"), "new", newer)
	if b, err := os.ReadFile(filepath.Join(Dir(), "terminal-input", "outcome.uncertain")); err != nil || string(b) != "keep" {
		t.Fatal("input receipt lost", err)
	}
}

func TestPurposeFreshPublication(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := InitializePurposeStorage(); err != nil {
		t.Fatal(err)
	}
	if err := CheckStorageLayout(); err != nil {
		t.Fatal(err)
	}
	j, err := readWorkflowJournal()
	if err != nil || j == nil || j.Phase != purposeActive {
		t.Fatal(j, err)
	}
	if err := InitializePurposeStorage(); err != nil {
		t.Fatal(err)
	}
	again, err := readWorkflowJournal()
	if err != nil || again.proof != j.proof {
		t.Fatal("reinitialized existing home", err)
	}
	if err := ChangePurposeLayout(false, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	if err := CheckStorageLayout(); err != nil {
		t.Fatal(err)
	}
}

func TestPurposeServiceFenceBeforeParkAndAfterMove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	layoutFixture(t, "events.jsonl", "preserve")
	if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	if err := ChangeWorkflowLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	stop := func() error {
		if err := CheckStorageLayout(); err == nil {
			t.Fatal("parked while ordinary commands were still admitted")
		}
		return os.ErrPermission
	}
	if err := ChangePurposeLayoutWithServices(true, layoutWritersStopped, stop); err == nil {
		t.Fatal("ignored service refusal")
	}
	if err := CancelPurposePreparation(); err != nil {
		t.Fatal(err)
	}
	if err := CheckStorageLayout(); err == nil {
		t.Fatal("cancel lifted the fence before profiles returned")
	}
	if err := ReleaseWorkflowServiceFence(); err != nil {
		t.Fatal(err)
	}
	if err := CheckStorageLayout(); err != nil {
		t.Fatal(err)
	}
	if err := ChangePurposeLayoutWithServices(true, layoutWritersStopped, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	if err := CheckStorageLayout(); err == nil {
		t.Fatal("admitted before service profiles were returned")
	}
	if err := ReleaseWorkflowServiceFence(); err != nil {
		t.Fatal(err)
	}
	if err := CheckStorageLayout(); err != nil {
		t.Fatal(err)
	}
}
