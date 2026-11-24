package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestGSS003CommandsGateInterruptedLayout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_LANG", "en")
	now := time.Now().Unix()
	if err := events.AppendChecked(events.Record{Ts: now, Event: "layout-fixture-one"}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	busy := errors.New("writer appeared")
	if err := state.ChangeStorageLayout(true, func() error {
		calls++
		if calls == 2 {
			return busy
		}
		return nil
	}); !errors.Is(err, busy) {
		t.Fatalf("expected a pending layout change, got %v", err)
	}
	output := captureStdout(t, func() {
		if code := Run([]string{"doctor", "--storage-layout"}); code != 0 {
			t.Fatalf("layout preview exit %d", code)
		}
	})
	if !strings.Contains(output, "Transition pending") || !strings.Contains(output, "layout-v2/events/events.jsonl") {
		t.Fatalf("layout preview: %q", output)
	}
	for _, args := range [][]string{{"events"}, {"codex"}, {"doctor", "--fix"}} {
		if code := Run(args); code != 1 {
			t.Fatalf("pending layout %v: exit %d", args, code)
		}
	}
	if err := state.ChangeStorageLayout(true, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := events.AppendChecked(events.Record{Ts: now, Event: "layout-fixture-two"}); err != nil {
		t.Fatal(err)
	}
	got := events.Read(60, now)
	if len(got) != 2 || got[0].Event != "layout-fixture-one" || got[1].Event != "layout-fixture-two" || got[0].Seq == 0 || got[1].Seq != got[0].Seq+1 {
		t.Fatalf("events or monotonic sequence lost: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(state.Dir(), "events.jsonl")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("flat event file reappeared: %v", err)
	}
	if f := measureStateStorage(state.Dir()); !f.complete || f.working == 0 {
		t.Fatalf("doctor did not recognize the relocated event log: %+v", f)
	}
}
