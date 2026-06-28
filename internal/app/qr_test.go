package app

import (
	"bytes"
	"strings"
	"testing"

	"rsc.io/qr"
)

// The branded QR knocks the gtmux mark into the center. Guard the two things
// that would silently break scanning: the finder patterns (corners) must be
// untouched, and the mark must land as the expected pane-grid silhouette.
func TestBrandMarkOverlay(t *testing.T) {
	code, err := qr.Encode(`{"v":2,"url":"https://gtmux-x.ccy.dev","enrollCode":"deadbeef","name":"mac"}`, qr.H)
	if err != nil {
		t.Fatal(err)
	}
	n := code.Size
	g := make([][]bool, n)
	before := make([][]bool, n)
	for y := 0; y < n; y++ {
		g[y] = make([]bool, n)
		before[y] = make([]bool, n)
		for x := 0; x < n; x++ {
			g[y][x] = code.Black(x, y)
			before[y][x] = g[y][x]
		}
	}
	overlayBrandMark(g)

	// finder patterns (7×7 corners) must survive the overlay.
	for _, c := range [][2]int{{0, 0}, {n - 7, 0}, {0, n - 7}} {
		for dy := 0; dy < 7; dy++ {
			for dx := 0; dx < 7; dx++ {
				if g[c[1]+dy][c[0]+dx] != before[c[1]+dy][c[0]+dx] {
					t.Fatalf("finder pixel changed at (%d,%d)", c[0]+dx, c[1]+dy)
				}
			}
		}
	}

	// the inner 7×7 must equal the brand mark; the 1-module ring around it light.
	kw := 9 // (7 + 2*margin) at unit=1
	x0, y0 := (n-kw)/2, (n-kw)/2
	mark := brandMark()
	for my := 0; my < 7; my++ {
		for mx := 0; mx < 7; mx++ {
			got := g[y0+1+my][x0+1+mx]
			if got != mark[my][mx] {
				t.Fatalf("mark mismatch at (%d,%d): got %v want %v", mx, my, got, mark[my][mx])
			}
		}
	}
	for x := x0; x < x0+kw; x++ {
		if g[y0][x] || g[y0+kw-1][x] {
			t.Fatalf("quiet ring not light at column %d", x)
		}
	}
}

// printBrandQR must emit a half-block QR with a quiet zone and never panic.
func TestPrintBrandQR(t *testing.T) {
	var b bytes.Buffer
	printBrandQR(&b, `{"v":2,"url":"https://gtmux-x.ccy.dev","enrollCode":"deadbeef","name":"mac"}`)
	out := b.String()
	if !strings.Contains(out, qrLit) {
		t.Fatal("expected lit half-blocks in output")
	}
	// first line is the all-light quiet-zone row (only the lit full block).
	first := strings.SplitN(out, "\n", 2)[0]
	if strings.Trim(first, qrLit) != "" {
		t.Fatalf("top quiet-zone row not all-light: %q", first)
	}
}
