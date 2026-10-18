package events

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReadSinceDoesNotMaterializeConsumedHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(Path()), 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(Path())
	if err != nil {
		t.Fatal(err)
	}
	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	const consumed = 4096
	summary := strings.Repeat("x", 1024)
	for i := 1; i <= consumed+1; i++ {
		if err := enc.Encode(Record{Seq: int64(i), Ts: 1000, Event: "Stop", Summary: summary}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	recs, gap := ReadSince(consumed)
	runtime.ReadMemStats(&after)
	if gap || len(recs) != 1 || recs[0].Seq != consumed+1 {
		t.Fatalf("tail=%v gap=%v", recs, gap)
	}
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("4096 consumed 1KiB records, one unread: allocated %d bytes", allocated)
	// Decoding the file still allocates strings. Allow generous decoder overhead,
	// but not the multiple whole-history Record slices formerly built before filtering.
	if allocated > 12<<20 {
		t.Fatalf("cursor read allocated %d bytes for a one-record tail; limit 12MiB", allocated)
	}
}

func TestReadSinceFiltersAcrossGenerationsWithoutChangingGaps(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(Path()), 0700); err != nil {
		t.Fatal(err)
	}
	for path, seqs := range map[string][]int64{rotatedPath(): {7, 9, 3}, Path(): {6, 8, 10, 0, 2}} {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		enc := json.NewEncoder(f)
		for _, seq := range seqs {
			if err := enc.Encode(Record{Seq: seq, Ts: 1000, Event: "Stop"}); err != nil {
				t.Fatal(err)
			}
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		cursor int64
		want   []int64
		gap    bool
	}{
		{5, []int64{6, 7, 8, 9, 10}, false},
		{3, []int64{6, 7, 8, 9, 10}, true},
		{0, []int64{2, 3, 6, 7, 8, 9, 10}, true},
		{-1, []int64{0, 2, 3, 6, 7, 8, 9, 10}, true},
		{10, nil, false},
	} {
		got, gap := ReadSince(tt.cursor)
		if gap != tt.gap || len(got) != len(tt.want) {
			t.Fatalf("cursor %d: count=%d gap=%v", tt.cursor, len(got), gap)
		}
		for i, r := range got {
			if r.Seq != tt.want[i] {
				t.Fatalf("cursor %d index %d: seq=%d", tt.cursor, i, r.Seq)
			}
		}
	}
}
