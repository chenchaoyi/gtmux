package hq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func setSnapshotPolicy(t *testing.T, c SnapshotPolicy) {
	t.Helper()
	p := filepath.Join(state.ConfigDir(), "config.json")
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(map[string]any{"hqSnapshots": c})
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func putRecord(t *testing.T, root, name string, b []byte) {
	t.Helper()
	p := filepath.Join(root, name)
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func snapshotTime() time.Time { return time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC) }

func TestAutomaticScopeAndExplicitFullExport(t *testing.T) {
	root := seedMemory(t)
	db, _ := json.Marshal(hqworkspace.Empty())
	for n, b := range map[string][]byte{"PERSONAL.md": []byte("personal rules"), "workspace/records.json": db, "notes/custom.json": []byte(`{"unique":true}`), "notes/reports/run/FINAL.md": []byte("key conclusion"), "notes/reports/run/SUMMARY.json": []byte(`{"result":"passed"}`), "notes/reports/run/raw.json": []byte(`{"raw":true}`), "notes/scan-relays.json": []byte(`[]`), "notes/demo-relays-closeout.json": []byte(`[]`), "notes/demo-relays-closure.json": []byte(`[]`), "notes/retained-relays-closure.json": []byte(`[]`), "notes/image.png": []byte("synthetic attachment")} {
		putRecord(t, root, n, b)
	}
	c := DefaultSnapshotPolicy()
	c.Include = []string{"notes/reports/run/raw.json", "notes/retained-relays-closure.json"}
	setSnapshotPolicy(t, c)
	p, w, e := Snapshot(snapshotTime())
	if e != nil || !w {
		t.Fatalf("snapshot %v %v", w, e)
	}
	a, e := readMemoryArchive(p, "")
	if e != nil {
		t.Fatal(e)
	}
	for _, n := range []string{"PERSONAL.md", "workspace/records.json", "notes/custom.json", "notes/board.md", "notes/reports/run/FINAL.md", "notes/reports/run/SUMMARY.json", "notes/reports/run/raw.json", "notes/retained-relays-closure.json"} {
		if _, ok := a.files[n]; !ok {
			t.Errorf("unique record missing: %s", n)
		}
	}
	for _, n := range []string{"notes/scan-relays.json", "notes/image.png", "notes/demo-relays-closeout.json", "notes/demo-relays-closure.json"} {
		if _, ok := a.files[n]; ok {
			t.Errorf("process item in automatic recovery: %s", n)
		}
	}
	full := filepath.Join(t.TempDir(), "full.tar.gz")
	if _, e := ExportMemory(full); e != nil {
		t.Fatal(e)
	}
	all, e := readMemoryArchive(full, "")
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"notes/image.png", "notes/demo-relays-closeout.json", "notes/demo-relays-closure.json"} {
		if _, ok := all.files[name]; !ok {
			t.Fatalf("explicit full export silently filtered %s", name)
		}
	}
	putRecord(t, root, "notes/board.md", []byte("new live board"))
	moved, e := ImportMemory(p)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(filepath.Join(moved, "notes/image.png")); e != nil {
		t.Fatal("core restore lost displaced original attachments")
	}
	b, _ := os.ReadFile(filepath.Join(root, "PERSONAL.md"))
	if string(b) != "personal rules" {
		t.Fatal("personal rules not restored")
	}
}

func TestAutomaticRetentionDoesNotAdoptLegacy(t *testing.T) {
	root := seedMemory(t)
	if e := os.MkdirAll(snapshotDir(), 0700); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 14; i++ {
		putRecord(t, snapshotDir(), fmt.Sprintf("hq-2026-10-%02d-legacy.tar.gz", i+1), []byte("unverified historical point"))
	}
	for i := 0; i < 7; i++ {
		putRecord(t, root, "notes/board.md", []byte(fmt.Sprintf("state %d", i)))
		if _, _, e := Snapshot(snapshotTime().Add(time.Duration(i) * 24 * time.Hour)); e != nil {
			t.Fatal(e)
		}
	}
	o := ReadSnapshotOverview()
	if o.ManagedPoints != 3 || o.LegacyArchives != 14 {
		t.Fatalf("retention %+v", o)
	}
	for i := 0; i < 14; i++ {
		b, _ := os.ReadFile(filepath.Join(snapshotDir(), fmt.Sprintf("hq-2026-10-%02d-legacy.tar.gz", i+1)))
		if string(b) != "unverified historical point" {
			t.Fatal("legacy archive touched")
		}
	}
	before := len(Snapshots())
	_ = os.Chtimes(filepath.Join(root, "notes/board.md"), snapshotTime(), snapshotTime())
	if _, w, e := Snapshot(snapshotTime().Add(9 * 24 * time.Hour)); e != nil || w {
		t.Fatalf("unchanged content re-packed across days: %v %v", w, e)
	}
	if len(Snapshots()) != before {
		t.Fatal("dedup changed retention")
	}
}

func TestSnapshotFailureAndDisableKeepRecovery(t *testing.T) {
	root := seedMemory(t)
	p, w, e := Snapshot(snapshotTime())
	if e != nil || !w {
		t.Fatal(e)
	}
	original, _ := os.ReadFile(p)
	huge := filepath.Join(root, "notes/unique-large.txt")
	f, e := os.Create(huge)
	if e != nil {
		t.Fatal(e)
	}
	_ = f.Truncate(archiveFileMaxBytes + 1)
	_ = f.Close()
	if _, _, e := Snapshot(snapshotTime().Add(time.Hour)); e == nil {
		t.Fatal("oversize unique note silently omitted")
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(after, original) {
		t.Fatal("failure removed prior recovery point")
	}
	out := filepath.Join(t.TempDir(), "full.tar.gz")
	_ = os.WriteFile(out, []byte("existing destination"), 0600)
	if _, e := ExportMemory(out); e == nil {
		t.Fatal("unimportable export accepted")
	}
	b, _ := os.ReadFile(out)
	if string(b) != "existing destination" {
		t.Fatal("failed export replaced destination")
	}
	var stream bytes.Buffer
	if _, e := WriteMemoryArchive(&stream); e == nil || stream.Len() != 0 {
		t.Fatal("stream sent bytes before export validation")
	}
	encrypted := out + ".age"
	_ = os.WriteFile(encrypted, []byte("previous encrypted copy"), 0600)
	if _, _, e := ExportMemoryEncrypted(encrypted, "synthetic-passphrase"); e == nil {
		t.Fatal("oversized encrypted export accepted")
	}
	b, _ = os.ReadFile(encrypted)
	if string(b) != "previous encrypted copy" {
		t.Fatal("encrypted destination replaced")
	}
	c := DefaultSnapshotPolicy()
	c.Enabled = false
	setSnapshotPolicy(t, c)
	if _, w, e := Snapshot(snapshotTime().Add(2 * time.Hour)); e != nil || w {
		t.Fatal("disabled snapshot wrote")
	}
	if _, e := os.Stat(p); e != nil {
		t.Fatal("disable pruned recovery")
	}
}

func TestBudgetAndCorruptionPreserveOldPoints(t *testing.T) {
	root := seedMemory(t)
	c := DefaultSnapshotPolicy()
	c.MaxMB = 1
	setSnapshotPolicy(t, c)
	p, _, e := Snapshot(snapshotTime())
	if e != nil {
		t.Fatal(e)
	}
	original, _ := os.ReadFile(p)
	payload := make([]byte, 1200*1024)
	_, _ = rand.New(rand.NewSource(7)).Read(payload)
	putRecord(t, root, "notes/unique.bin", payload)
	if _, _, e := Snapshot(snapshotTime().Add(time.Hour)); e == nil {
		t.Fatal("budget overflow accepted")
	}
	b, _ := os.ReadFile(p)
	if !bytes.Equal(b, original) {
		t.Fatal("budget failure deleted old point")
	}
	_ = os.Remove(filepath.Join(root, "notes/unique.bin"))
	bad := append([]byte(nil), original...)
	bad[len(bad)-1] ^= 1
	_ = os.WriteFile(p, bad, 0600)
	if _, e := ImportMemory(p); e == nil {
		t.Fatal("bad CRC restored")
	}
	putRecord(t, root, "notes/board.md", []byte("changed"))
	if _, _, e := Snapshot(snapshotTime().Add(2 * time.Hour)); e != nil {
		t.Fatal(e)
	}
	b, _ = os.ReadFile(p)
	if !bytes.Equal(b, bad) {
		t.Fatal("corrupt/unverified old package silently pruned")
	}
}

func TestConcurrentSnapshotPublishesOnePoint(t *testing.T) {
	seedMemory(t)
	c := DefaultSnapshotPolicy()
	c.IntervalMinutes = 0
	setSnapshotPolicy(t, c)
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, e := Snapshot(snapshotTime()); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if len(Snapshots()) != 1 {
		t.Fatal("duplicate concurrent recovery points")
	}
}

func TestExportEntryAndDestinationBounds(t *testing.T) {
	root := seedMemory(t)
	if _, e := ExportMemory(filepath.Join(root, "notes/export.tar.gz")); e == nil {
		t.Fatal("self-containing export allowed")
	}
	for i := 0; i < archiveMaxEntries; i++ {
		putRecord(t, root, fmt.Sprintf("notes/generated/%05d.json", i), nil)
	}
	if _, e := ExportMemory(filepath.Join(t.TempDir(), "too-many.tar.gz")); e == nil {
		t.Fatal("entry cap only enforced on import")
	}
}

func TestExportValidatesRestoredWorkspaceGrowth(t *testing.T) {
	root := seedMemory(t)
	d := hqworkspace.Empty()
	d.Messages["message"] = hqworkspace.Message{ID: "message", Delivery: "pending"}
	b, _ := json.Marshal(d)
	m := d.Messages["message"]
	m.Body = string(bytes.Repeat([]byte("x"), (32<<20)-len(b)-1))
	d.Messages["message"] = m
	b, _ = json.Marshal(d)
	if len(b) > 32<<20 {
		t.Fatal("fixture exceeds source bound")
	}
	putRecord(t, root, "workspace/records.json", b)
	target := filepath.Join(t.TempDir(), "existing.tar.gz")
	_ = os.WriteFile(target, []byte("previous recovery"), 0600)
	if _, e := ExportMemory(target); e == nil {
		t.Fatal("published a source whose pending-message restore transformation exceeds the workspace limit")
	}
	after, _ := os.ReadFile(target)
	if string(after) != "previous recovery" {
		t.Fatal("failed restore validation replaced old export")
	}
}
