package hqwake

import (
	"os"
	"regexp"
	"strconv"
	"sync"
	"testing"
)

func TestCHH002_ConsumedDistinguishesMissingFromCorruptAndSerializesConsume(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := Consumed(); got != WatermarkUnset {
		t.Fatalf("missing watermark = %d, want WatermarkUnset (%d)", got, WatermarkUnset)
	}

	// Empty or corrupt watermark must read as 0 rather than WatermarkUnset (-1) so it
	// never triggers bootstrap adoption of the tail.
	if err := os.MkdirAll(dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "   \n", "corrupt", "-9"} {
		if err := os.WriteFile(watermarkPath(), []byte(bad), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := Consumed(); got != 0 {
			t.Fatalf("Consumed() on %q = %d, want 0", bad, got)
		}
	}

	var wg sync.WaitGroup
	for i := int64(1); i <= 64; i++ {
		wg.Add(1)
		go func(seq int64) {
			defer wg.Done()
			Consume(seq)
		}(i)
	}
	wg.Wait()
	if got := Consumed(); got != 64 {
		t.Fatalf("Consumed() after concurrent updates = %d, want 64", got)
	}
}

func TestCHH002_ConcurrentAddOutcomeAndConsumeTickLoseZeroOutcomes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const (
		writers   = 8
		perWriter = 80
		wantTotal = writers * perWriter
	)
	countRe := regexp.MustCompile(`(\d+) done`)
	parseDone := func(line string) (int, bool) {
		if line == "" {
			return 0, true
		}
		m := countRe.FindStringSubmatch(line)
		if len(m) != 2 {
			return 0, false
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, false
		}
		return n, true
	}

	var wg sync.WaitGroup
	doneWriters := make(chan struct{})
	totalConsumed := 0
	badLine := ""
	var consumeMu sync.Mutex

	var consumerWG sync.WaitGroup
	consumerWG.Add(1)
	go func() {
		defer consumerWG.Done()
		seq := int64(10)
		for {
			select {
			case <-doneWriters:
				return
			default:
				if line := ConsumeTick(1_700_000_000, seq); line != "" {
					n, ok := parseDone(line)
					consumeMu.Lock()
					if !ok && badLine == "" {
						badLine = line
					}
					totalConsumed += n
					consumeMu.Unlock()
					seq++
				}
			}
		}
	}()

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				AddOutcome("done")
			}
		}()
	}
	wg.Wait()
	close(doneWriters)
	consumerWG.Wait()

	if rem := ConsumeTick(1_700_000_100, 999); rem != "" {
		n, ok := parseDone(rem)
		if !ok && badLine == "" {
			badLine = rem
		}
		totalConsumed += n
	}
	if badLine != "" {
		t.Fatalf("unexpected tick line %q", badLine)
	}
	if totalConsumed != wantTotal {
		t.Fatalf("consumed outcomes = %d, want %d (zero outcomes lost)", totalConsumed, wantTotal)
	}
}
