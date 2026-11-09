package radar

import (
	"bytes"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPWR001ProducerSharesFreshSnapshotAndKeepsBoundaries(t *testing.T) {
	now := time.Unix(100, 0)
	home := "first"
	var loads atomic.Int32
	c := &snapshotCache{now: func() time.Time { return now }, scope: func() string { return home }, load: func() ([]byte, error) { loads.Add(1); return []byte(`{"generation":"fallback"}`), nil }}
	producer := func() ([]byte, error) { loads.Add(1); return []byte(`{"generation":"tick"}`), nil }
	first, err := c.refresh(producer)
	if err != nil {
		t.Fatal(err)
	}
	first[0] = 'x'
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			b, e := c.read()
			if e != nil || string(b) != `{"generation":"tick"}` {
				t.Errorf("snapshot %s %v", b, e)
			}
		})
	}
	wg.Wait()
	if loads.Load() != 1 {
		t.Fatalf("REST repeated producer work: %d", loads.Load())
	}
	if _, err = c.refresh(producer); err != nil || loads.Load() != 2 {
		t.Fatal("SSE producer was suppressed by REST TTL")
	}
	home = "second"
	b, e := c.read()
	if e != nil || string(b) != `{"generation":"fallback"}` || loads.Load() != 3 {
		t.Fatal("profile scope crossed")
	}
	now = now.Add(snapshotTTL)
	c.read()
	if loads.Load() != 4 {
		t.Fatal("expiry did not recollect")
	}
	if _, e = c.refresh(func() ([]byte, error) { return nil, errors.New("fixture failure") }); e == nil || c.body != nil {
		t.Fatal("failed producer retained success bytes")
	}
	c.read()
	if loads.Load() != 5 {
		t.Fatal("failed producer disabled fallback")
	}
	c.refresh(func() ([]byte, error) { return bytes.Repeat([]byte{'x'}, snapshotByteCap+1), nil })
	if c.body != nil {
		t.Fatal("oversize retained")
	}
}

func TestPWR001ConcurrentProducerCannotBeOverwrittenByOlderREST(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	c := &snapshotCache{now: time.Now, load: func() ([]byte, error) { close(entered); <-release; return []byte("old"), nil }}
	done := make(chan struct{})
	go func() { defer close(done); c.read() }()
	<-entered
	producerDone := make(chan struct{})
	go func() { defer close(producerDone); c.refresh(func() ([]byte, error) { return []byte("new"), nil }) }()
	close(release)
	<-done
	<-producerDone
	b, e := c.read()
	if e != nil || string(b) != "new" {
		t.Fatalf("older load replaced producer: %s %v", b, e)
	}
}
