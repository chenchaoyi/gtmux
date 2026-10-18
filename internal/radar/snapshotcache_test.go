package radar

import (
	"bytes"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestServeSnapshotSharesWorkAndDoesNotExposeMutableStorage(t *testing.T) {
	now := time.Unix(10, 0)
	var calls atomic.Int32
	c := &snapshotCache{now: func() time.Time { return now }, load: func() ([]byte, error) { calls.Add(1); return []byte("[]"), nil }}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, err := c.read()
			if err != nil || string(b) != "[]" {
				t.Errorf("snapshot %s %v", b, err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("concurrent readers recollected %d times", calls.Load())
	}
	b, _ := c.read()
	b[0] = 'x'
	b, _ = c.read()
	if string(b) != "[]" {
		t.Fatal("caller changed the cached snapshot")
	}
	now = now.Add(2 * time.Second)
	c.read()
	if calls.Load() != 2 {
		t.Fatal("freshness deadline was not enforced")
	}
	c.load = func() ([]byte, error) { return nil, errors.New("unavailable") }
	now = now.Add(2 * time.Second)
	if b, err := c.read(); err == nil || b != nil {
		t.Fatal("load failure became stale success")
	}
	c.load = func() ([]byte, error) { return bytes.Repeat([]byte{'x'}, snapshotByteCap+1), nil }
	c.read()
	if c.body != nil {
		t.Fatal("oversized snapshot remained retained")
	}
}

func TestServeSnapshotDoesNotCrossProfileHomes(t *testing.T) {
	home := "first"
	calls := 0
	c := &snapshotCache{scope: func() string { return home }, now: time.Now, load: func() ([]byte, error) { calls++; return []byte(home), nil }}
	c.read()
	home = "second"
	b, err := c.read()
	if err != nil || string(b) != "second" || calls != 2 {
		t.Fatalf("cross-profile cache: %q %d %v", b, calls, err)
	}
}
