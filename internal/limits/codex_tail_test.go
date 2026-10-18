package limits

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func quotaRecord(pct int, reset int64) string {
	return fmt.Sprintf(`{"payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":%d,"window_minutes":300,"resets_at":%d},"plan_type":"plus"}}}`, pct, reset)
}

// Sparse prefixes avoid copying real logs or allocating their entire contents.
func quotaSparseLog(t testing.TB, size int64, tail string) string {
	t.Helper()
	if size < int64(len(tail)) {
		t.Fatal("fixture tail exceeds size")
	}
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte(tail), size-int64(len(tail))); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCodexWindowsInTailRecords(t *testing.T) {
	now := time.Unix(1_788_700_000, 0)
	older := quotaRecord(10, now.Unix()+3600)
	newer := quotaRecord(25, now.Unix()+3600)
	for _, tc := range []struct {
		name  string
		body  string
		found bool
		pct   int
	}{
		{"latest-valid", older + "\n" + newer + "\n" + `{"payload":{"rate_limits":null}}` + "\n{broken}", true, 25},
		{"no-final-newline", newer, true, 25},
		{"empty", "", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := quotaSparseLog(t, int64(len(tc.body)), tc.body)
			wins, found, at := codexWindowsIn(path, now)
			if found != tc.found || found && (len(wins) != 1 || wins[0].PctUsed != tc.pct || wins[0].PlanType != "plus" || at.IsZero()) {
				t.Fatalf("windows=%+v found=%v at=%v", wins, found, at)
			}
		})
	}
	t.Run("latest-expired-does-not-revive-older", func(t *testing.T) {
		body := older + "\n" + quotaRecord(30, now.Unix()-1)
		path := quotaSparseLog(t, int64(len(body)), body)
		wins, found, _ := codexWindowsIn(path, now)
		if !found || len(wins) != 0 {
			t.Fatalf("windows=%+v found=%v", wins, found)
		}
	})
	t.Run("exact-tail-boundary", func(t *testing.T) {
		body := newer + "\n" + strings.Repeat(" ", codexTailBytes-len(newer)-1)
		for _, prefix := range []string{"", "prefix\n"} {
			path := quotaSparseLog(t, int64(len(prefix)+len(body)), prefix+body)
			wins, found, _ := codexWindowsIn(path, now)
			if !found || len(wins) != 1 || wins[0].PctUsed != 25 {
				t.Fatalf("prefix=%q windows=%+v found=%v", prefix, wins, found)
			}
		}
	})
	t.Run("cut-record-cannot-masquerade-as-valid-JSON", func(t *testing.T) {
		// The tail happens to start at a valid-looking object, but the full
		// JSONL record has a non-JSON prefix. The old slice-after-ReadFile
		// implementation incorrectly accepted this fragment as a quota record.
		body := "invalid-prefix" + newer + "\n" + strings.Repeat(" ", codexTailBytes-len(newer)-1)
		path := quotaSparseLog(t, int64(len(body)), body)
		if wins, found, _ := codexWindowsIn(path, now); found {
			t.Fatalf("accepted truncated record: %+v", wins)
		}
	})
	t.Run("cut-record-then-complete-record", func(t *testing.T) {
		tail := "partial record\n" + newer
		path := quotaSparseLog(t, 2*codexTailBytes, tail)
		wins, found, _ := codexWindowsIn(path, now)
		if !found || len(wins) != 1 || wins[0].PctUsed != 25 {
			t.Fatalf("windows=%+v found=%v", wins, found)
		}
	})
	t.Run("oversize-record-without-newline", func(t *testing.T) {
		path := quotaSparseLog(t, 2*codexTailBytes, newer)
		if wins, found, _ := codexWindowsIn(path, now); found {
			t.Fatalf("accepted unterminated oversized record: %+v", wins)
		}
	})
}

type quotaReadProbe struct {
	reader    io.ReaderAt
	before    func()
	calls     int
	requested int
}

func (r *quotaReadProbe) ReadAt(p []byte, off int64) (int, error) {
	r.calls++
	r.requested += len(p)
	if r.before != nil {
		r.before()
		r.before = nil
	}
	return r.reader.ReadAt(p, off)
}

type quotaShortReader struct{ err error }

func (r quotaShortReader) ReadAt(p []byte, _ int64) (int, error) {
	return copy(p, "short"), r.err
}

func TestReadCodexTailSnapshot(t *testing.T) {
	t.Run("append-is-left-for-next-poll", func(t *testing.T) {
		path := quotaSparseLog(t, 2*codexTailBytes, "\noriginal\n")
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			t.Fatal(err)
		}
		probe := &quotaReadProbe{reader: f, before: func() {
			if _, err := f.WriteAt([]byte("appended\n"), fi.Size()); err != nil {
				t.Fatal(err)
			}
		}}
		got, err := readCodexTail(probe, fi.Size())
		if err != nil || string(got) != "original\n" || probe.calls != 1 || probe.requested != codexTailBytes+1 {
			t.Fatalf("tail=%q err=%v probe=%+v", got, err, probe)
		}
	})
	t.Run("truncate-rejects-short-snapshot", func(t *testing.T) {
		path := quotaSparseLog(t, 2*codexTailBytes, "\noriginal\n")
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		fi, err := f.Stat()
		if err != nil {
			t.Fatal(err)
		}
		probe := &quotaReadProbe{reader: f, before: func() {
			if err := f.Truncate(fi.Size() - 1); err != nil {
				t.Fatal(err)
			}
		}}
		if got, err := readCodexTail(probe, fi.Size()); err == nil || got != nil || probe.calls != 1 {
			t.Fatalf("tail=%q err=%v calls=%d", got, err, probe.calls)
		}
	})
	for _, readErr := range []error{nil, io.EOF, errors.New("read failure")} {
		t.Run(fmt.Sprintf("short-read-%v", readErr), func(t *testing.T) {
			got, err := readCodexTail(quotaShortReader{readErr}, 100)
			if got != nil || err == nil || readErr != nil && !errors.Is(err, readErr) || readErr == nil && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("tail=%q err=%v", got, err)
			}
		})
	}
}

func TestCodexWindowsInAllocationBound(t *testing.T) {
	now := time.Unix(1_788_700_000, 0)
	tail := "\n" + quotaRecord(25, now.Unix()+3600) + "\n"
	for _, size := range []int64{32 << 10, 24 << 20} {
		path := quotaSparseLog(t, size, tail)
		runtime.GC() // Test-process measurement only; never the running server.
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		wins, found, _ := codexWindowsIn(path, now)
		runtime.ReadMemStats(&after)
		allocated := after.TotalAlloc - before.TotalAlloc
		t.Logf("file_bytes=%d allocated_bytes=%d heap_inuse_after=%d", size, allocated, after.HeapInuse)
		if !found || len(wins) != 1 || wins[0].PctUsed != 25 {
			t.Fatal("missing reading")
		}
		// This guards the actual parser, not only the helper or allocation count.
		// Restoring os.ReadFile before slicing fails on the 24MiB sparse log.
		if allocated > 4*codexTailBytes {
			t.Fatalf("allocated %d bytes reading %d-byte log, want <=%d", allocated, size, 4*codexTailBytes)
		}
	}
}

func BenchmarkCodexWindowsInTail(b *testing.B) {
	now := time.Unix(1_788_700_000, 0)
	tail := "\n" + quotaRecord(25, now.Unix()+3600) + "\n"
	for _, tc := range []struct {
		name string
		size int64
	}{{"small", 32 << 10}, {"large", 24 << 20}} {
		b.Run(tc.name, func(b *testing.B) {
			path := quotaSparseLog(b, tc.size, tail)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				wins, found, _ := codexWindowsIn(path, now)
				if !found || len(wins) != 1 {
					b.Fatal("missing reading")
				}
			}
		})
	}
}
