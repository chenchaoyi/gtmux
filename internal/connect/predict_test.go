package connect

import (
	"strings"
	"testing"
	"time"
)

// armed builds a predictor already in the "slow link, cooked line" state where predicting
// is correct: enabled, a cursor seen, not alt-screen, and a measured slow round-trip.
func armed() *Predictor {
	p := NewPredictor(true)
	p.OnInput([]byte("x"))                               // starts the rtt clock (nothing predicted yet)
	p.lastSend = time.Now().Add(-300 * time.Millisecond) // pretend that key went out 300ms ago
	p.OnCursor(Cursor{X: 5, Y: 0, Alt: false})           // closes the measurement → rtt ≈ 300ms
	return p
}

func TestPredictorDrawsUnderlinedAndErasesOnOutput(t *testing.T) {
	p := armed()
	out := string(p.OnInput([]byte("ab")))
	if !strings.Contains(out, "\x1b[4m") || !strings.Contains(out, "a") || !strings.Contains(out, "b") {
		t.Fatalf("expected underlined a+b, got %q", out)
	}
	// Authoritative output arrives → the two predicted cells are erased EXACTLY (back 2,
	// two spaces, back 2) — never ESC[K, which would eat real text to the right.
	er := string(p.OnOutput())
	if er != "\x1b[2D  \x1b[2D" {
		t.Errorf("erase = %q; want back-2 + 2 spaces + back-2", er)
	}
	if got := string(p.OnOutput()); got != "" {
		t.Errorf("second erase should be a no-op, got %q", got)
	}
}

func TestPredictorBackspace(t *testing.T) {
	p := armed()
	p.OnInput([]byte("ab"))
	if got := string(p.OnInput([]byte{0x7f})); got != "\x1b[1D \x1b[1D" {
		t.Errorf("backspace rub-out = %q", got)
	}
	// Backspacing past our own predictions would erase CONFIRMED text → stop guessing.
	p.OnInput([]byte{0x7f}) // pops the last predicted char
	p.OnInput([]byte{0x7f}) // nothing left → end the epoch
	if p.epoch {
		t.Error("backspacing past predictions must end the epoch")
	}
}

func TestPredictorStateChangingKeysEndEpoch(t *testing.T) {
	for _, k := range []struct {
		name string
		b    byte
	}{{"CR", '\r'}, {"LF", '\n'}, {"ESC", 0x1b}, {"Ctrl-C", 0x03}, {"Tab", '\t'}, {"multibyte", 0xe4}} {
		p := armed()
		p.OnInput([]byte("ab"))
		out := string(p.OnInput([]byte{k.b}))
		if !strings.Contains(out, "\x1b[2D") {
			t.Errorf("%s: expected the outstanding predictions erased, got %q", k.name, out)
		}
		if p.epoch {
			t.Errorf("%s must end the prediction epoch", k.name)
		}
		// and it stays off until an authoritative cursor re-arms it
		if got := string(p.OnInput([]byte("z"))); got != "" {
			t.Errorf("%s: must not predict after the epoch ended, got %q", k.name, got)
		}
		p.OnCursor(Cursor{X: 0, Y: 0})
		if !p.epoch {
			t.Errorf("%s: a fresh cursor must re-arm the epoch", k.name)
		}
	}
}

func TestPredictorGates(t *testing.T) {
	// disabled (the default) → pure passthrough, never a byte drawn
	off := NewPredictor(false)
	off.OnCursor(Cursor{X: 1})
	if got := string(off.OnInput([]byte("abc"))); got != "" {
		t.Errorf("disabled predictor must draw nothing, got %q", got)
	}

	// fast link → nothing predicted (no flicker, no risk)
	fast := NewPredictor(true)
	fast.OnInput([]byte("x"))
	fast.lastSend = time.Now().Add(-5 * time.Millisecond)
	fast.OnCursor(Cursor{X: 1})
	if got := string(fast.OnInput([]byte("a"))); got != "" {
		t.Errorf("fast link must not predict, got %q", got)
	}

	// alt-screen (full-screen TUI like vim) → never predict
	alt := armed()
	alt.OnCursor(Cursor{X: 1, Alt: true})
	if got := string(alt.OnInput([]byte("a"))); got != "" {
		t.Errorf("alt-screen must not predict, got %q", got)
	}

	// no cursor from the server (an OLD server) → never predict
	noCur := NewPredictor(true)
	noCur.rttMS = 300
	if got := string(noCur.OnInput([]byte("a"))); got != "" {
		t.Errorf("without an authoritative cursor there must be no prediction, got %q", got)
	}
}

func TestPredictorStopsRunawayGuessing(t *testing.T) {
	p := armed()
	p.OnInput([]byte(strings.Repeat("a", predictMaxPending+10)))
	if len(p.pending) > predictMaxPending {
		t.Errorf("pending = %d; must be capped at %d", len(p.pending), predictMaxPending)
	}
}

func TestClassifyKey(t *testing.T) {
	cases := []struct {
		b    byte
		want predictKind
	}{
		{'a', kindPrintable}, {' ', kindPrintable}, {'~', kindPrintable},
		{0x7f, kindBackspace}, {0x08, kindBackspace},
		{'\r', kindStateChange}, {0x1b, kindStateChange}, {0x03, kindStateChange},
		{0x00, kindStateChange}, {0xe4, kindStateChange}, // NUL + multibyte lead
	}
	for _, c := range cases {
		if got := classifyKey(c.b); got != c.want {
			t.Errorf("classifyKey(%#x) = %v; want %v", c.b, got, c.want)
		}
	}
}
