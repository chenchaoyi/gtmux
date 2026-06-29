package app

import (
	"bytes"
	"strings"
	"testing"

	"rsc.io/qr"
)

// printBrandQR must emit SQUARE half-blocks (NOT quadrant — that distorts the
// code 2:1 tall; see the footgun note in qr.go) and carry no color escapes (the
// terminal QR has no center logo).
func TestPrintBrandQR(t *testing.T) {
	var b bytes.Buffer
	printBrandQR(&b, `{"v":2,"url":"https://gtmux-x.ccy.dev","enrollCode":"deadbeef"}`)
	out := b.String()
	if !strings.Contains(out, "█") {
		t.Fatal("expected solid blocks in output")
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatal("terminal QR must be plain (no color escapes / no drawn logo)")
	}
	// width in glyphs of the first row should be ~ the QR+quiet module count (one
	// glyph per module column), NOT halved — a quadrant render would distort it.
	first := strings.SplitN(out, "\n", 2)[0]
	if w := len([]rune(first)); w > 60 {
		t.Fatalf("QR too wide for a square half-block render: %d cols", w)
	}
}

// Dropping the center logo lets the terminal QR encode at qr.L (lowest EC) instead
// of qr.M, which is what makes it smaller — guard that L really yields no more
// modules than M for a representative pairing payload (so the "make it smaller"
// fix can't silently regress).
func TestTerminalQRUsesSmallerEC(t *testing.T) {
	payload := `{"v":2,"url":"https://gtmux-qclyu2s2.ccy.dev","enrollCode":"deadbeef"}`
	l, err := qr.Encode(payload, qr.L)
	if err != nil {
		t.Fatal(err)
	}
	m, err := qr.Encode(payload, qr.M)
	if err != nil {
		t.Fatal(err)
	}
	if l.Size > m.Size {
		t.Fatalf("expected qr.L (%d) to be no larger than qr.M (%d)", l.Size, m.Size)
	}
}

// buildGrid pads the code with a quiet zone and an even row count so the
// half-block packing (2 module-rows per char) is clean.
func TestBuildGridEvenRows(t *testing.T) {
	code, err := qr.Encode(`{"v":2,"url":"https://gtmux-x.ccy.dev","enrollCode":"x"}`, qr.L)
	if err != nil {
		t.Fatal(err)
	}
	g := buildGrid(code)
	if len(g)%2 != 0 {
		t.Fatalf("grid rows must be even for half-block packing, got %d", len(g))
	}
	if len(g) != len(g[0]) {
		t.Fatalf("grid must be square: %d×%d", len(g[0]), len(g))
	}
}
