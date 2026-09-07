package hq

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The supervisor's memory is 6.1 MB accumulated over three months on the machine this
// was written for, and none of it was recoverable: no export, no snapshot, not a git
// repo, no Time Machine destination, no cloud folder. It survived every context reset
// and would not have survived one `rm -rf`.

func seedMemory(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := MemoryRoot()
	for path, body := range map[string]string{
		"AGENTS.md":             "# playbook v33",
		"LOCAL.md":              "the operator's own charter",
		"notes/board.md":        "## ① 现状\n| pane | loc |\n|---|---|\n| %7 | a |",
		"knowledge/index.jsonl": `{"id":"pitfalls/x","title":"a lesson"}`,
	} {
		p := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestExportAndImportRoundTrip(t *testing.T) {
	root := seedMemory(t)
	dst := filepath.Join(t.TempDir(), "hq.tar.gz")
	n, err := ExportMemory(dst)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if n <= 0 {
		t.Fatal("export wrote nothing")
	}

	// Wipe the memory, the way an `rm -rf` would.
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportMemory(dst); err != nil {
		t.Fatalf("import: %v", err)
	}
	for path, want := range map[string]string{
		"LOCAL.md":       "the operator's own charter",
		"notes/board.md": "## ① 现状",
	} {
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatalf("%s did not come back: %v", path, err)
		}
		if !strings.Contains(string(b), want) {
			t.Errorf("%s = %q", path, b)
		}
	}
}

func TestImportMovesAnExistingMemoryAsideRatherThanOverwriting(t *testing.T) {
	// Restoring is done in a hurry and usually on the wrong assumption. The one thing
	// this must never do is turn "I restored last week's board" into "and I destroyed
	// today's".
	root := seedMemory(t)
	dst := filepath.Join(t.TempDir(), "hq.tar.gz")
	if _, err := ExportMemory(dst); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes", "board.md"), []byte("TODAY'S WORK"), 0o644); err != nil {
		t.Fatal(err)
	}

	moved, err := ImportMemory(dst)
	if err != nil {
		t.Fatal(err)
	}
	if moved == "" {
		t.Fatal("the existing memory was overwritten in place")
	}
	b, err := os.ReadFile(filepath.Join(moved, "notes", "board.md"))
	if err != nil || !strings.Contains(string(b), "TODAY'S WORK") {
		t.Errorf("the displaced memory did not survive at %s: %v", moved, err)
	}
}

func TestImportRefusesSomethingThatIsNotAMemory(t *testing.T) {
	// Importing writes over the operator's charter, so "it was a .tar.gz" is not
	// enough of a check: a mistyped path must fail loudly.
	seedMemory(t)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "node_modules/left-pad/index.js", Size: 2, Mode: 0o644})
	_, _ = tw.Write([]byte("hi"))
	_ = tw.Close()
	_ = gz.Close()
	junk := filepath.Join(t.TempDir(), "junk.tar.gz")
	if err := os.WriteFile(junk, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ImportMemory(junk); err == nil {
		t.Fatal("imported an archive carrying no supervisor memory")
	}
	// And the real memory is untouched.
	if _, err := os.Stat(filepath.Join(MemoryRoot(), "LOCAL.md")); err != nil {
		t.Errorf("a refused import disturbed the existing memory: %v", err)
	}
}

func TestImportRefusesAPathThatEscapes(t *testing.T) {
	// A tar can name `../../.ssh/authorized_keys`, and this one is unpacked from a file
	// somebody was handed.
	seedMemory(t)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "notes/board.md", Size: 1, Mode: 0o644})
	_, _ = tw.Write([]byte("x"))
	_ = tw.WriteHeader(&tar.Header{Name: "../../escaped", Size: 1, Mode: 0o644})
	_, _ = tw.Write([]byte("x"))
	_ = tw.Close()
	_ = gz.Close()
	evil := filepath.Join(t.TempDir(), "evil.tar.gz")
	if err := os.WriteFile(evil, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportMemory(evil); err == nil {
		t.Fatal("accepted an archive that escapes the destination")
	}
}

func TestExportRefusesWhenThereIsNoMemory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := ExportMemory(filepath.Join(t.TempDir(), "x.tar.gz")); err == nil {
		t.Error("exported a memory that does not exist")
	}
}

// --- snapshots --------------------------------------------------------------

func TestSnapshotWritesOncePerChange(t *testing.T) {
	seedMemory(t)
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)

	if _, wrote, err := Snapshot(now); err != nil || !wrote {
		t.Fatalf("first snapshot: wrote=%v err=%v", wrote, err)
	}
	// A quiet machine must not burn fourteen identical copies and evict real history.
	if _, wrote, _ := Snapshot(now.Add(time.Hour)); wrote {
		t.Error("snapshotted again with nothing changed")
	}
	if len(Snapshots()) != 1 {
		t.Errorf("got %d snapshots, want 1", len(Snapshots()))
	}

	// A day where the memory really moved replaces that day's snapshot.
	if err := os.WriteFile(filepath.Join(MemoryRoot(), "notes", "board.md"), []byte("new state"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, wrote, _ := Snapshot(now.Add(2 * time.Hour)); !wrote {
		t.Error("did not snapshot after the memory changed")
	}
	if len(Snapshots()) != 1 {
		t.Errorf("a second same-day snapshot accumulated: %d", len(Snapshots()))
	}
}

func TestSnapshotKeepsAFortnightAndPrunesTheRest(t *testing.T) {
	seedMemory(t)
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < SnapshotKeep+6; i++ {
		if err := os.WriteFile(filepath.Join(MemoryRoot(), "notes", "board.md"),
			[]byte("day "+time.Duration(i).String()), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Snapshot(base.AddDate(0, 0, i)); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(Snapshots()); n != SnapshotKeep {
		t.Errorf("kept %d snapshots, want %d", n, SnapshotKeep)
	}
	// The NEWEST survive: a loss noticed last week must still be recoverable, and the
	// oldest is the one nobody wants.
	if !strings.Contains(Snapshots()[0], base.AddDate(0, 0, SnapshotKeep+5).Format("2006-01-02")) {
		t.Errorf("the newest snapshot was pruned: %v", Snapshots()[0])
	}
}

func TestSnapshotIsNotStoredInsideTheThingItProtects(t *testing.T) {
	// The likeliest loss is the home going away. A backup kept inside it goes too.
	seedMemory(t)
	if _, _, err := Snapshot(time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, s := range Snapshots() {
		if strings.HasPrefix(s, MemoryRoot()) {
			t.Errorf("snapshot %s lives inside the memory it protects", s)
		}
	}
	// And it survives the deletion that matters.
	if err := os.RemoveAll(MemoryRoot()); err != nil {
		t.Fatal(err)
	}
	if len(Snapshots()) != 1 {
		t.Error("the snapshot went with the home it was protecting")
	}
}

func TestSnapshotNoSupervisor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, wrote, err := Snapshot(time.Now()); err != nil || wrote {
		t.Errorf("a machine with no supervisor: wrote=%v err=%v, want a quiet no-op", wrote, err)
	}
}

func TestReadMemoryStateMeasuresWhatIsAtRisk(t *testing.T) {
	seedMemory(t)
	s := ReadMemoryState()
	if !s.Exists || s.Files != 4 || s.Bytes == 0 {
		t.Fatalf("state = %+v", s)
	}
	if s.Snapshots != 0 || s.LastSnap != "" {
		t.Errorf("reported a snapshot before any was taken: %+v", s)
	}
	if _, _, err := Snapshot(time.Now()); err != nil {
		t.Fatal(err)
	}
	if s := ReadMemoryState(); s.Snapshots != 1 || s.LastSnapAt == 0 {
		t.Errorf("after one snapshot: %+v", s)
	}
}

func TestPlaybookBackupsAreNotHoardedForever(t *testing.T) {
	// Keeping every upgrade was right while there were three. The real HQ home had
	// TWENTY, which reads as a folder of debris a reader has to scroll past to find
	// their own files. The backup exists so one bad upgrade can be undone.
	seedMemory(t)
	base := time.Now().Add(-40 * time.Hour)
	// Named so the LEXICAL order disagrees with the chronological one: v9 sorts after
	// v22, so a prune that sorted by name would keep the oldest.
	for i, v := range []int{2, 5, 7, 9, 10, 22, 25, 33} {
		p := hqInstructionsPath() + fmt.Sprintf(".bak-v%d-en", v)
		if err := os.WriteFile(p, []byte("playbook v"+fmt.Sprint(v)), 0o644); err != nil {
			t.Fatal(err)
		}
		at := base.Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(p, at, at); err != nil {
			t.Fatal(err)
		}
	}
	prunePlaybookBackups()

	left, _ := filepath.Glob(hqInstructionsPath() + ".bak-v*")
	if len(left) != playbookBackupsKeep {
		t.Fatalf("kept %d backups, want %d: %v", len(left), playbookBackupsKeep, left)
	}
	// The NEWEST survive, which is what "undo the last upgrade" needs.
	for _, want := range []int{22, 25, 33} {
		if _, err := os.Stat(hqInstructionsPath() + fmt.Sprintf(".bak-v%d-en", want)); err != nil {
			t.Errorf("v%d should have been kept: %v", want, err)
		}
	}
}

func TestPruneLeavesAHandfulAlone(t *testing.T) {
	seedMemory(t)
	for _, v := range []int{31, 32} {
		if err := os.WriteFile(hqInstructionsPath()+fmt.Sprintf(".bak-v%d-en", v), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prunePlaybookBackups()
	if left, _ := filepath.Glob(hqInstructionsPath() + ".bak-v*"); len(left) != 2 {
		t.Errorf("pruned below the keep count: %v", left)
	}
}

// `hq --memory --json` is what the menu bar reads, so its shape is a contract between two
// surfaces — and the off-machine sentence in particular must be the SAME sentence the
// doctor row prints, not a paraphrase that can drift from it.
func TestMemoryJSONCarriesWhatASurfaceNeeds(t *testing.T) {
	seedMemory(t)
	out := captureStdout(t, func() { printMemoryState(true) })
	var got memoryJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, out)
	}
	if !got.Exists || got.Files != 4 || got.Bytes == 0 {
		t.Errorf("state = %+v", got)
	}
	if got.OffMachine == "" {
		t.Error("no off-machine sentence — the surface would silently claim nothing about protection")
	}
	if got.Root == "" {
		t.Error("no root path")
	}
}

func TestMemoryJSONOnAMachineWithNoSupervisor(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	out := captureStdout(t, func() { printMemoryState(true) })
	var got memoryJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if got.Exists {
		t.Error("claimed a memory on a machine with none")
	}
}
