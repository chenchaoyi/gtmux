package app

import (
	"bytes"
	"strings"
	"testing"

	"rsc.io/qr"
)

// The branded QR stamps the 2×2 brand mark into the center. Guard that the
// finder patterns (corners) stay clear of the logo and that the mark lands with
// the app's colors: top-left cyan, the other three grey, on a white patch.
func TestBrandGridLogo(t *testing.T) {
	code, err := qr.Encode(`{"v":2,"url":"https://gtmux-x.ccy.dev","enrollCode":"deadbeef","name":"mac"}`, qr.H)
	if err != nil {
		t.Fatal(err)
	}
	g := buildGrid(code, true)
	size := len(g)

	// recompute the logo geometry (mirrors overlayBrandMark).
	const cell, gap, margin = 2, 2, 2
	knock := 2*cell + gap + 2*margin // 10
	x0 := ((size - knock) / 2) &^ 1
	y0 := ((size - knock) / 2) &^ 1

	// patch corners are white.
	for _, p := range [][2]int{{x0, y0}, {x0 + knock - 1, y0 + knock - 1}} {
		if g[p[1]][p[0]] != cWhite {
			t.Fatalf("patch corner (%d,%d) not white: %d", p[0], p[1], g[p[1]][p[0]])
		}
	}
	// the four cells.
	cx0, cx1 := x0+margin, x0+margin+cell+gap
	cy0, cy1 := y0+margin, y0+margin+cell+gap
	if g[cy0][cx0] != cCyan {
		t.Fatalf("top-left cell not cyan: %d", g[cy0][cx0])
	}
	for _, p := range [][2]int{{cx1, cy0}, {cx0, cy1}, {cx1, cy1}} {
		if g[p[1]][p[0]] != cGrey {
			t.Fatalf("cell (%d,%d) not grey: %d", p[0], p[1], g[p[1]][p[0]])
		}
	}

	// finder patterns (data corners at the quiet-zone offset, 7×7) must be logo-free.
	const qz = 2
	for _, c := range [][2]int{{qz, qz}, {size - qz - 7, qz}, {qz, size - qz - 7}} {
		for dy := 0; dy < 7; dy++ {
			for dx := 0; dx < 7; dx++ {
				if v := g[c[1]+dy][c[0]+dx]; v != cDark && v != cLight {
					t.Fatalf("logo color leaked into finder at (%d,%d): %d", c[0]+dx, c[1]+dy, v)
				}
			}
		}
	}
}

// printBrandQR must emit quadrant blocks, the cyan accent escape, and stay
// compact (≈ half the module width, since each char packs a 2×2 block).
func TestPrintBrandQR(t *testing.T) {
	var b bytes.Buffer
	printBrandQR(&b, `{"v":2,"url":"https://gtmux-x.ccy.dev","enrollCode":"deadbeef","name":"mac"}`)
	out := b.String()
	if !strings.Contains(out, cellColor[cCyan]) {
		t.Fatal("expected the cyan accent escape in output")
	}
	if !strings.Contains(out, "█") {
		t.Fatal("expected solid blocks in output")
	}
	// width in glyphs of the first row should be ~ half the QR+quiet module size.
	first := strings.SplitN(out, "\n", 2)[0]
	if w := len([]rune(stripANSI(first))); w > 60 {
		t.Fatalf("QR too wide for a quadrant render: %d cols", w)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
