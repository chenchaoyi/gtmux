package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestTrimFileTail(t *testing.T) {
	dir := t.TempDir()

	// Missing file → no-op, no error.
	if err := trimFileTail(filepath.Join(dir, "nope.log"), 100, 50); err != nil {
		t.Fatalf("missing file should be a no-op, got %v", err)
	}

	// Under cap → untouched.
	small := filepath.Join(dir, "small.log")
	if err := os.WriteFile(small, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := trimFileTail(small, 1<<20, 1<<10); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(small); string(b) != "line1\nline2\n" {
		t.Fatalf("under-cap file changed: %q", b)
	}

	// Over cap → trimmed to the tail, starting on a clean line boundary.
	big := filepath.Join(dir, "big.log")
	var sb strings.Builder
	for i := 0; i < 5000; i++ {
		sb.WriteString("some log line here that takes up space\n")
	}
	orig := sb.String()
	if err := os.WriteFile(big, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := trimFileTail(big, 8<<10, 2<<10); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(big)
	if int64(len(got)) > 2<<10 {
		t.Fatalf("trimmed size %d exceeds keepBytes", len(got))
	}
	if len(got) == 0 || got[0] == '\n' {
		t.Fatalf("trimmed head should start on a clean line, got %q…", got[:min(20, len(got))])
	}
	if !bytes.HasSuffix([]byte(orig), got) {
		t.Fatalf("trimmed content must be a suffix of the original")
	}
	// Every retained line must be a whole original line (no partial leading line).
	if !strings.HasSuffix(string(got), "\n") {
		t.Fatalf("tail should end on a newline")
	}
	for _, ln := range strings.Split(strings.TrimSuffix(string(got), "\n"), "\n") {
		if ln != "some log line here that takes up space" {
			t.Fatalf("partial line retained: %q", ln)
		}
	}
}

func TestPruneDir(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)

	// Missing dir → no-op.
	if err := pruneDir(filepath.Join(t.TempDir(), "nope"), time.Hour, 100, now); err != nil {
		t.Fatalf("missing dir should be a no-op, got %v", err)
	}

	dir := t.TempDir()
	write := func(name string, size int, age time.Duration) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, bytes.Repeat([]byte("x"), size), 0o644); err != nil {
			t.Fatal(err)
		}
		mt := now.Add(-age)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// Age pruning: older-than-maxAge is deleted, recent survives.
	old := write("old.bin", 10, 48*time.Hour)
	recent := write("recent.bin", 10, time.Minute)
	if err := pruneDir(dir, 24*time.Hour, 1<<30, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("aged-out file should be deleted")
	}
	if _, err := os.Stat(recent); err != nil {
		t.Fatalf("recent file should survive, got %v", err)
	}

	// Size pruning: all within maxAge but over the total cap → oldest-first deleted.
	dir2 := t.TempDir()
	writeIn := func(d, name string, size int, age time.Duration) string {
		p := filepath.Join(d, name)
		_ = os.WriteFile(p, bytes.Repeat([]byte("y"), size), 0o644)
		mt := now.Add(-age)
		_ = os.Chtimes(p, mt, mt)
		return p
	}
	oldest := writeIn(dir2, "a.bin", 100, 3*time.Hour)
	mid := writeIn(dir2, "b.bin", 100, 2*time.Hour)
	newest := writeIn(dir2, "c.bin", 100, 1*time.Hour)
	// cap 250 → total 300, must drop the single oldest (100) to reach 200 ≤ 250.
	if err := pruneDir(dir2, 24*time.Hour, 250, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldest); !os.IsNotExist(err) {
		t.Fatalf("oldest should be deleted first")
	}
	for _, p := range []string{mid, newest} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s should survive under the cap, got %v", filepath.Base(p), err)
		}
	}

	// Fresh small dir → nothing deleted.
	dir3 := t.TempDir()
	keep := writeIn(dir3, "keep.bin", 10, time.Minute)
	if err := pruneDir(dir3, 24*time.Hour, 1<<30, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("small fresh dir should be untouched, got %v", err)
	}
}

// TestDiskHygieneSweep verifies the wired sweep caps ALL four launchd logs (incl.
// restore.log), ages out a dead pane's churn marker while keeping a fresh one, and
// respects its ≤ 1/30-min throttle.
func TestDiskHygieneSweep(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	base := state.Dir()
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	now := int64(2_000_000)

	// Over-cap logs, including the 4th (restore.log) the first cut missed.
	big := bytes.Repeat([]byte("x"), int(logMaxBytes)+4096)
	for _, name := range []string{"serve.log", "tunnel.log", "selftunnel.log", "restore.log"} {
		if err := os.WriteFile(filepath.Join(base, name), big, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A dead pane's churn marker (old mtime) and a live one (fresh) in a marker dir.
	frameDir := filepath.Join(base, "frame")
	if err := os.MkdirAll(frameDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dead := filepath.Join(frameDir, "%dead")
	live := filepath.Join(frameDir, "%live")
	_ = os.WriteFile(dead, []byte("stale"), 0o644)
	_ = os.WriteFile(live, []byte("fresh"), 0o644)
	oldT := time.Unix(now, 0).Add(-30 * 24 * time.Hour)
	freshT := time.Unix(now, 0).Add(-time.Minute)
	_ = os.Chtimes(dead, oldT, oldT)
	_ = os.Chtimes(live, freshT, freshT)

	diskHygieneSweep(now)

	for _, name := range []string{"serve.log", "tunnel.log", "selftunnel.log", "restore.log"} {
		fi, err := os.Stat(filepath.Join(base, name))
		if err != nil || fi.Size() > logMaxBytes {
			t.Fatalf("%s not capped: size=%v err=%v", name, fi, err)
		}
	}
	if _, err := os.Stat(dead); !os.IsNotExist(err) {
		t.Fatalf("dead pane's stale marker should be aged out")
	}
	if _, err := os.Stat(live); err != nil {
		t.Fatalf("live pane's fresh marker must survive, got %v", err)
	}

	// Throttle: a second sweep within the interval is a no-op even if a log is big again.
	if err := os.WriteFile(filepath.Join(base, "serve.log"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	diskHygieneSweep(now + hygieneInterval - 1)
	if fi, _ := os.Stat(filepath.Join(base, "serve.log")); fi == nil || fi.Size() <= logMaxBytes {
		t.Fatalf("throttled sweep should NOT re-cap the log")
	}
	// Past the interval it runs again.
	diskHygieneSweep(now + hygieneInterval + 1)
	if fi, _ := os.Stat(filepath.Join(base, "serve.log")); fi == nil || fi.Size() > logMaxBytes {
		t.Fatalf("post-interval sweep should re-cap the log")
	}
}

// TestRowDiskUsage exercises the doctor storage sentinel's three tiers via sparse files
// (Truncate reports a large logical size without consuming disk).
func TestRowDiskUsage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	base := state.Dir()
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}

	// Small footprint → OK.
	_ = os.WriteFile(filepath.Join(base, "small"), []byte("hi"), 0o644)
	if r := rowDiskUsage(); r.status != stOK {
		t.Fatalf("small state dir → stOK, got %d (%s)", r.status, r.value)
	}

	// Amber: a sparse file just over 500 MB.
	amber := filepath.Join(base, "amber.bin")
	f, _ := os.Create(amber)
	_ = f.Truncate(diskAmberBytes + (1 << 20))
	_ = f.Close()
	if r := rowDiskUsage(); r.status != stRec {
		t.Fatalf("500MB+ → stRec (amber), got %d (%s)", r.status, r.value)
	}

	// Red: grow it past 2 GB.
	f2, _ := os.OpenFile(amber, os.O_RDWR, 0o644)
	_ = f2.Truncate(diskRedBytes + (1 << 20))
	_ = f2.Close()
	if r := rowDiskUsage(); r.status != stMiss {
		t.Fatalf("2GB+ → stMiss (red), got %d (%s)", r.status, r.value)
	}
}
