package hqwake

import "testing"

// The watermark's three invariants, each one a way HQ could otherwise be told it is caught
// up when it is not.
func TestWatermark(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// UNSET is distinct from 0: a fresh install must be able to adopt the stream end
	// instead of being told it is thousands of events behind, and 0 ("consumed nothing on
	// an empty stream") is a real position it must not be confused with.
	if got := Consumed(); got != WatermarkUnset {
		t.Fatalf("fresh state: Consumed() = %d, want WatermarkUnset", got)
	}
	if !Consume(0) {
		t.Fatal("Consume(0) on an unset watermark must record the position")
	}
	if got := Consumed(); got != 0 {
		t.Fatalf("Consumed() = %d, want 0", got)
	}

	// MONOTONIC: a stale or out-of-order writer must never rewind HQ's position, which
	// would re-raise events it has already judged.
	Consume(50)
	if Consume(20) {
		t.Error("Consume(20) after 50 reported a move")
	}
	if got := Consumed(); got != 50 {
		t.Errorf("Consumed() = %d, want the watermark held at 50", got)
	}
	if Consume(-1) {
		t.Error("a negative sequence must be ignored")
	}
}

// ConsumeRead is where the "what counts as consumption" rule lives: only a read CONTIGUOUS
// with what HQ has already consumed. A read starting past the watermark saw the tail and
// skipped the middle — accepting it would drop that range silently, which is the exact
// failure the watermark exists to prevent.
func TestConsumeReadRequiresContiguity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Bootstrap: with no watermark yet, HQ's first pull defines its position.
	if !ConsumeRead(40, 60) || Consumed() != 60 {
		t.Fatalf("first read on an unset watermark: Consumed() = %d, want 60", Consumed())
	}
	// Contiguous (starts at or before the watermark) → consumed.
	if !ConsumeRead(60, 75) || Consumed() != 75 {
		t.Errorf("contiguous read: Consumed() = %d, want 75", Consumed())
	}
	if !ConsumeRead(10, 80) || Consumed() != 80 {
		t.Errorf("a read starting further back is still contiguous: Consumed() = %d, want 80", Consumed())
	}
	// Skip-ahead → the debt stands.
	if ConsumeRead(90, 120) || Consumed() != 80 {
		t.Errorf("skip-ahead read: Consumed() = %d, want it held at 80", Consumed())
	}
}

// The unread class is the completeness net, so it must queue like a standing condition:
// last to drain (never ahead of a blocked agent) and first to be evicted at the cap, since
// it re-fires on its own while a decision knock does not.
func TestUnreadIsStandingPriority(t *testing.T) {
	line := Line(ClassUnread, "7 unconsumed", "pull: gtmux events --since-seq 6653 --json")
	if got := PriorityOf(line); got != PriorityStanding {
		t.Errorf("PriorityOf(%q) = %d, want PriorityStanding", line, got)
	}
}

func TestUnreadConfigDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := Load()
	if cfg.UnreadDebounceSec != 120 || cfg.UnreadRepeatSec != 300 {
		t.Errorf("defaults = %ds/%ds, want 120s debounce / 300s repeat",
			cfg.UnreadDebounceSec, cfg.UnreadRepeatSec)
	}
}
