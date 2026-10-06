package events

import (
	"os"
	"sync"
	"testing"
	"time"
)

// collect runs Follow until stop and gathers what it emits, by sequence number.
type collector struct {
	mu   sync.Mutex
	seqs []int64
}

func (c *collector) add(r Record) {
	c.mu.Lock()
	c.seqs = append(c.seqs, r.Seq)
	c.mu.Unlock()
}

func (c *collector) waitFor(t *testing.T, n int) []int64 {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		c.mu.Lock()
		got := len(c.seqs)
		c.mu.Unlock()
		if got >= n {
			break
		}
	}
	time.Sleep(600 * time.Millisecond) // two more polls: anything emitted twice shows up
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]int64(nil), c.seqs...)
}

func wantEach(t *testing.T, got []int64, want ...int64) {
	t.Helper()
	count := map[int64]int{}
	for _, s := range got {
		count[s]++
	}
	for _, s := range want {
		if count[s] != 1 {
			t.Errorf("seq %d emitted %d times (all: %v)", s, count[s], got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("emitted %v, want each of %v once", got, want)
	}
}

// %12's reproduction (2026-10-06): a record appended while the replay is being emitted
// fell between the replay and the stream and was never delivered; a later append was.
func TestFollowDeliversAnAppendMadeDuringTheReplay(t *testing.T) {
	tinyCap(t, 1)
	now := time.Now().Unix()
	Append(Record{Ts: now, Event: "initial", State: "idle", Loc: "a"}) // seq 1
	c := &collector{}
	stop := make(chan struct{})
	bridged := false
	go Follow(60, now, func(r Record) {
		c.add(r)
		if !bridged {
			bridged = true
			Append(Record{Ts: now, Event: "bridge", State: "idle", Loc: "b"}) // seq 2, during the replay
		}
	}, stop)
	time.Sleep(300 * time.Millisecond)
	Append(Record{Ts: now, Event: "control", State: "idle", Loc: "c"}) // seq 3, after it
	got := c.waitFor(t, 3)
	close(stop)
	wantEach(t, got, 1, 2, 3)
}

// A record appended after the open and before the replay reads is in both; it is
// emitted once.
func TestFollowEmitsARecordBothReadOnce(t *testing.T) {
	tinyCap(t, 1)
	now := time.Now().Unix()
	Append(Record{Ts: now, Event: "initial", State: "idle", Loc: "a"}) // seq 1
	saved := afterFollowOpen
	afterFollowOpen = func() { Append(Record{Ts: now, Event: "between", State: "idle", Loc: "b"}) } // seq 2
	t.Cleanup(func() { afterFollowOpen = saved })
	c := &collector{}
	stop := make(chan struct{})
	go Follow(60, now, c.add, stop)
	got := c.waitFor(t, 2)
	close(stop)
	wantEach(t, got, 1, 2)
}

// With no log yet, the first file that appears is new from its first line: a record
// written before the follower's next poll is delivered, not skipped as "already there".
func TestFollowStreamsALogThatAppearsLater(t *testing.T) {
	tinyCap(t, 1)
	if _, err := os.Stat(Path()); !os.IsNotExist(err) {
		t.Fatalf("the log exists already (%v)", err)
	}
	c := &collector{}
	stop := make(chan struct{})
	go Follow(0, time.Now().Unix(), c.add, stop)
	time.Sleep(50 * time.Millisecond)
	Append(Record{Ts: time.Now().Unix(), Event: "first", State: "idle", Loc: "a"})
	got := c.waitFor(t, 1)
	close(stop)
	wantEach(t, got, CurrentSeq())
}
