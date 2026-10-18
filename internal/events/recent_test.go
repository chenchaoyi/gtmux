package events

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRecentFiltersBeforeLimitAndSpansRotation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(Path()), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(path string, rows ...Record) {
		t.Helper()
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		for _, r := range rows {
			if err := json.NewEncoder(f).Encode(r); err != nil {
				t.Fatal(err)
			}
		}
	}
	write(rotatedPath(), Record{Seq: 1, Ts: 100, Event: "yes"}, Record{Seq: 2, Ts: 101, Event: "no"}, Record{Seq: 3, Ts: 102, Event: "yes"})
	write(Path(), Record{Seq: 4, Ts: 103, Event: "no"}, Record{Seq: 5, Ts: 104, Event: "yes"}, Record{Seq: 6, Ts: 1, Event: "yes"})
	rows := Recent(20, 110, 3, func(r Record) bool { return r.Event == "yes" })
	var seqs []int64
	for _, r := range rows {
		seqs = append(seqs, r.Seq)
	}
	if !reflect.DeepEqual(seqs, []int64{5, 3, 1}) {
		t.Fatalf("matching recent sequence: %v", seqs)
	}
	if rows := Recent(20, 110, 0, nil); rows == nil || len(rows) != 0 {
		t.Fatal("empty result must be an array")
	}
	if err := os.Remove(Path()); err != nil {
		t.Fatal(err)
	}
	if rows := Recent(20, 110, 1, nil); len(rows) != 1 || rows[0].Seq != 3 {
		t.Fatal("missing active generation lost rotated tail")
	}
}

func TestRecentBoundsLongRecordsAndPreservesUTF8Tail(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(Path()), 0700); err != nil {
		t.Fatal(err)
	}
	body := strings.Repeat("x", 2<<20) + "\n" + strings.Repeat("broken\n", 12000) + "{\"seq\":9,\"summary\":\"对话\"}"
	if err := os.WriteFile(Path(), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	rows := Recent(0, 0, 2, nil)
	if len(rows) != 1 || rows[0].Seq != 9 || rows[0].Summary != "对话" {
		t.Fatalf("recent bounded tail: %#v", rows)
	}
}

func BenchmarkRecentLargeJournal(b *testing.B) {
	b.Setenv("HOME", b.TempDir())
	if err := os.MkdirAll(filepath.Dir(Path()), 0700); err != nil {
		b.Fatal(err)
	}
	f, err := os.Create(Path())
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 80000; i++ {
		fmt.Fprintf(f, "{\"seq\":%d,\"summary\":%q}\n", i, strings.Repeat("x", 256))
	}
	f.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if len(Recent(0, 0, 200, nil)) != 200 {
			b.Fatal("lost recent records")
		}
	}
}
