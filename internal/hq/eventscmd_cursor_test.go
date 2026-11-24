package hq

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
)

func TestHCR001SkipAheadReadProvidesSavedCursor(t *testing.T) {
	asHQ(t)
	now := time.Now().Unix()
	for i := 0; i < 4; i++ {
		events.Append(events.Record{Ts: now, Event: "Stop", State: "idle", Loc: "web:1.0"})
	}
	hqwake.Consume(1)

	var rc int
	preview, warning := captureBoth(t, func() { rc = CmdEvents([]string{"--since-seq", "3", "--json"}) })
	var tail events.Record
	if err := json.Unmarshal([]byte(preview), &tail); err != nil || tail.Seq != 4 || rc != 0 {
		t.Fatalf("preview should still return event 4 with status zero: rc=%d out=%q err=%v", rc, preview, err)
	}
	if got := hqwake.Consumed(); got != 1 {
		t.Errorf("a skip-ahead read moved the watermark to %d, want it left at 1", got)
	}
	if !strings.Contains(warning, "gtmux events --since-seq 1 --json") {
		t.Fatalf("preview must name the actual saved cursor on stderr, got %q", warning)
	}

	read, warning := captureBoth(t, func() { rc = CmdEvents([]string{"--since-seq", "1", "--json"}) })
	var seqs []int64
	for _, line := range strings.Split(strings.TrimSpace(read), "\n") {
		var r events.Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("retry polluted JSON records: %v", err)
		}
		seqs = append(seqs, r.Seq)
	}
	if rc != 0 || strings.Contains(warning, "gtmux events --since-seq") || !slices.Equal(seqs, []int64{2, 3, 4}) || hqwake.Consumed() != 4 {
		t.Fatalf("retry did not consume the full delta: rc=%d seqs=%v watermark=%d stderr=%q", rc, seqs, hqwake.Consumed(), warning)
	}
}
