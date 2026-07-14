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
