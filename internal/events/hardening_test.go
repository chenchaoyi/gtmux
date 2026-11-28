package events

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCHH002_ConcurrentAppendAcrossRotationPreservesContiguousSequence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const (
		workers  = 8
		perWork  = 35
		smallCap = 4096
	)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start
			for i := 0; i < perWork; i++ {
				if err := appendWithCap(Record{
					Event:   "Stop",
					Pane:    "%1",
					Summary: strings.Repeat("x", 80),
				}, smallCap, false); err != nil {
					t.Errorf("appendWithCap: %v", err)
				}
			}
		}(w)
	}
	close(start)
	wg.Wait()

	cur := CurrentSeq()
	if cur != workers*perWork {
		t.Fatalf("CurrentSeq() = %d, want %d", cur, workers*perWork)
	}
	// Read the surviving tail across rotated + active segments and ensure strictly
	// contiguous ascending Seq numbers with zero false sequence-gap alarms.
	tailCursor := cur - 15
	recs, gap := ReadSince(tailCursor)
	if gap {
		t.Fatalf("unexpected sequence gap for recent cursor %d (recs=%d)", tailCursor, len(recs))
	}
	if len(recs) != 15 {
		t.Fatalf("len(recs) = %d, want 15", len(recs))
	}
	for i, r := range recs {
		want := tailCursor + int64(i) + 1
		if r.Seq != want {
			t.Fatalf("recs[%d].Seq = %d, want %d", i, r.Seq, want)
		}
	}
}

func TestCHH003_TailReadSinceAndReadSkipRotatedGenerationWhenCovered(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := int64(1_700_000_000)

	writeSeg := func(path string, fromSeq, toSeq, baseTs int64) {
		t.Helper()
		var buf []byte
		pad := strings.Repeat("p", 900)
		for seq := fromSeq; seq <= toSeq; seq++ {
			b, err := json.Marshal(Record{
				Ts:      baseTs + seq,
				Seq:     seq,
				Event:   "Stop",
				Pane:    "%2",
				Summary: pad,
			})
			if err != nil {
				t.Fatal(err)
			}
			buf = append(buf, append(b, '\n')...)
		}
		if err := os.MkdirAll(strings.TrimSuffix(path, "/"+strings.Split(path, "/")[len(strings.Split(path, "/"))-1]), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// Write >180 KiB in events.jsonl (seq 101..300) so multi-chunk backwards scanning is exercised.
	writeSeg(rotatedPath(), 1, 100, now-1000)
	writeSeg(Path(), 101, 300, now-1000)

	// Make rotatedPath unreadable so we prove ReadSince(250) and Read(20, now-700) never touch it.
	if err := os.Chmod(rotatedPath(), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(rotatedPath(), 0o600) })

	recs, gap := ReadSince(250)
	if gap || len(recs) != 50 || recs[0].Seq != 251 || recs[len(recs)-1].Seq != 300 {
		t.Fatalf("ReadSince(250) = len %d gap=%v first/last=%v, want 251..300 no gap", len(recs), gap, len(recs))
	}

	recent := Read(20, now-700) // cutoff = now-720 => seq >= 280
	if len(recent) != 21 || recent[0].Seq != 280 || recent[len(recent)-1].Seq != 300 {
		t.Fatalf("Read(20) = len %d, want 280..300", len(recent))
	}

	// Restore rotatedPath permissions and verify cross-generation fallback + gap detection.
	if err := os.Chmod(rotatedPath(), 0o600); err != nil {
		t.Fatal(err)
	}
	cross, crossGap := ReadSince(95)
	if crossGap || len(cross) != 205 || cross[0].Seq != 96 || cross[len(cross)-1].Seq != 300 {
		t.Fatalf("cross-generation ReadSince(95) = len %d gap=%v", len(cross), crossGap)
	}

	// Introduce a gap between rotated (1..80) and active (101..300).
	writeSeg(rotatedPath(), 1, 80, now-1000)
	_, gapDetected := ReadSince(75)
	if !gapDetected {
		t.Fatal("expected gap=true when seq 81..100 are missing between generations")
	}
	_ = time.Now()
}
