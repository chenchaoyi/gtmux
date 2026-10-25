package events

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestJournalAndSequenceArePrivateBeforeAppend(t *testing.T) {
	tinyCap(t, 20)
	previous := syscall.Umask(0)
	defer syscall.Umask(previous)
	now := time.Now().Unix()
	for _, stale := range []bool{false, true} {
		if stale {
			for _, p := range []string{Path(), seqPath(), state.Dir()} {
				if err := os.Chmod(p, 0755); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := AppendChecked(Record{Ts: now, Event: "Stop"}); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{Path(), seqPath()} {
			info, err := os.Stat(p)
			if err != nil || info.Mode().Perm()&0077 != 0 {
				t.Fatalf("stale=%v %s: %v %v", stale, p, info, err)
			}
		}
	}
	records := Read(0, now)
	if len(records) != 2 || records[0].Seq != 1 || records[1].Seq != 2 {
		t.Fatalf("journal contract changed: %+v", records)
	}
}
