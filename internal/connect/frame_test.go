package connect

import (
	"bytes"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		op   byte
		data []byte
	}{
		{OpInput, []byte("ls -la\r")},
		{OpOutput, []byte{0x1b, '[', '2', 'J'}}, // an ANSI escape stays intact
		{OpPause, nil},
		{OpOutput, []byte{}}, // empty payload
	} {
		frame := Encode(tc.op, tc.data)
		op, payload, ok := Decode(frame)
		if !ok || op != tc.op || !bytes.Equal(payload, tc.data) {
			t.Fatalf("round-trip op=%c: got (%c,%q,%v)", tc.op, op, payload, ok)
		}
	}
}

func TestDecodeEmptyFrame(t *testing.T) {
	if _, _, ok := Decode(nil); ok {
		t.Error("empty frame should not decode")
	}
}

func TestResize(t *testing.T) {
	frame := EncodeResize(120, 40)
	op, payload, ok := Decode(frame)
	if !ok || op != OpResize {
		t.Fatalf("resize frame op = %c", op)
	}
	cols, rows, ok := DecodeResize(payload)
	if !ok || cols != 120 || rows != 40 {
		t.Fatalf("DecodeResize = (%d,%d,%v), want (120,40,true)", cols, rows, ok)
	}
	// Non-positive / malformed dims are rejected.
	if _, _, ok := DecodeResize([]byte(`{"cols":0,"rows":40}`)); ok {
		t.Error("zero cols should be rejected")
	}
	if _, _, ok := DecodeResize([]byte("not json")); ok {
		t.Error("bad json should be rejected")
	}
}

// OpCursor (attach-predictive-echo): the server-sent tmux cursor the client uses as the
// authoritative reconcile point for predictions.
func TestCursor(t *testing.T) {
	frame := EncodeCursor(12, 3, true)
	op, payload, ok := Decode(frame)
	if !ok || op != OpCursor {
		t.Fatalf("Decode → op=%q ok=%v; want OpCursor", op, ok)
	}
	c, ok := DecodeCursor(payload)
	if !ok || c.X != 12 || c.Y != 3 || !c.Alt {
		t.Errorf("DecodeCursor = %+v ok=%v; want {12 3 true}", c, ok)
	}
	// a cooked-line (non-alt) cursor round-trips too
	if c2, ok := DecodeCursor(EncodeCursor(0, 0, false)[1:]); !ok || c2.Alt || c2.X != 0 {
		t.Errorf("DecodeCursor(non-alt) = %+v ok=%v", c2, ok)
	}
	// garbage / negative cells are "unknown", never a bogus cell
	if _, ok := DecodeCursor([]byte("not json")); ok {
		t.Error("bad JSON should not decode")
	}
	if _, ok := DecodeCursor([]byte(`{"x":-1,"y":0}`)); ok {
		t.Error("negative cell should not decode")
	}
}
