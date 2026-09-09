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
	const payload = `{"v":2,"url":"https://gtmux-x.ccy.dev","enrollCode":"deadbeef"}`
	var b bytes.Buffer
	printBrandQR(&b, payload)
	out := b.String()
	if !strings.Contains(out, "█") {
		t.Fatal("expected solid blocks in output")
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatal("terminal QR must be plain (no color escapes / no drawn logo)")
	}
	// The aspect is the whole point, so pin it against the grid that was rendered
	// rather than against a width budget. A half block is one module wide and two
	// modules tall: one glyph per module COLUMN, one line per two module ROWS.
	//
	// The budget this replaces (`w > 60`) failed in the direction that matters. A
	// quadrant render HALVES the columns, so the code it was named after — 19 cols
	// where 38 are right — sailed through it, and PR #179 would ship again.
	code, err := qr.Encode(payload, qr.L)
	if err != nil {
		t.Fatal(err)
	}
	g := buildGrid(code)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if w := len([]rune(lines[0])); w != len(g[0]) {
		t.Fatalf("%d glyphs across a %d-module row — a square render is one glyph per module column; "+
			"halving it is the quadrant distortion qr.go forbids", w, len(g[0]))
	}
	if len(lines) != len(g)/2 {
		t.Fatalf("%d lines for %d module rows — a half block packs exactly two rows per line",
			len(lines), len(g))
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
