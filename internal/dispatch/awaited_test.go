package dispatch

import "testing"

func TestAwaitedRegistry_RoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if IsAwaited("%9") {
		t.Fatalf("a fresh pane must not be awaited")
	}
	MarkAwaited("%9")
	if !IsAwaited("%9") {
		t.Fatalf("MarkAwaited should make the pane awaited")
	}
	ClearAwaited("%9")
	if IsAwaited("%9") {
		t.Fatalf("ClearAwaited should drop the marker")
	}
	// Clearing an unmarked pane / empty pane is a no-op, not a panic.
	ClearAwaited("%9")
	MarkAwaited("")
	if IsAwaited("") {
		t.Fatalf("an empty pane id is never awaited")
	}
}
