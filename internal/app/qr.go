package app

import (
	"fmt"
	"io"
	"strings"

	"rsc.io/qr"
)

// Terminal pairing QR — a clean, SQUARE half-block code with NO center mark.
//
// Rendered with HALF blocks (one char = 1 module wide × 2 modules tall). A
// terminal cell is ~1:2 (twice as tall as wide), so a half block makes each QR
// module render visually SQUARE.
//
// FOOTGUN — DO NOT "shrink" this by switching to quadrant blocks (2×2 modules per
// char). That halves the columns but renders each module at the cell's 1:2 ratio,
// so the WHOLE code comes out STRETCHED 2:1 tall — a distorted, ugly QR. We tried
// it (PR #179) and it was reverted. The code MUST stay square. The only way to
// make a square code smaller is to reduce the MODULE COUNT — shorter payload,
// lower error-correction level, or a smaller quiet zone — never the render aspect.
//
// NO center logo: a terminal QR is ASCII, so it can't carry the raster app icon,
// and a hand-drawn brand mark looked off — the branded center belongs to the
// graphical menu-bar pairing QR, where the real app icon can be composited.
// Dropping the mark also lets us encode at qr.L (the lowest error-correction
// level) for the smallest module count, since there's no center occlusion to
// recover from — so the printed code is meaningfully smaller while staying square.
//
// Polarity: a DARK module renders as empty (terminal bg shows through), a LIGHT
// module / quiet zone as a filled block in the default fg — so it scans the same
// way qrterminal renders.

// module cell kinds.
const (
	cDark  int8 = iota // empty (terminal bg)
	cLight             // filled, default fg (data + quiet zone)
)

// printBrandQR renders payload as a small, square half-block QR (no center mark).
// On encode failure it prints nothing rather than a broken code.
func printBrandQR(w io.Writer, payload string) {
	code, err := qr.Encode(payload, qr.L)
	if err != nil {
		return
	}
	renderHalfBlocks(w, buildGrid(code))
}

// buildGrid turns a QR code into a padded module grid (quiet zone + even row count
// for half-block packing) of cell kinds.
func buildGrid(code *qr.Code) [][]int8 {
	const qz = 2 // quiet-zone modules each side
	n := code.Size
	size := n + 2*qz
	if size%2 != 0 {
		size++ // pad to even so half-block (2 rows/char) packing is clean
	}
	g := make([][]int8, size)
	for y := 0; y < size; y++ {
		g[y] = make([]int8, size)
		for x := 0; x < size; x++ {
			mx, my := x-qz, y-qz
			if mx >= 0 && mx < n && my >= 0 && my < n && code.Black(mx, my) {
				g[y][x] = cDark
			} else {
				g[y][x] = cLight // data-light, quiet zone, and the even-pad edge
			}
		}
	}
	return g
}

// renderHalfBlocks prints the grid with one character per 1-module-wide,
// 2-module-tall cell (top row + bottom row), so modules render visually SQUARE.
// (Do NOT replace this with a quadrant render to save width — see the FOOTGUN note
// at the top of the file: it distorts the code 2:1 tall.)
func renderHalfBlocks(w io.Writer, g [][]int8) {
	size := len(g)
	var b strings.Builder
	for y := 0; y < size; y += 2 {
		for x := 0; x < size; x++ {
			top := g[y][x]
			bot := cLight // bottom defaults to light (the even-pad edge)
			if y+1 < size {
				bot = g[y+1][x]
			}
			td, bd := top != cDark, bot != cDark
			switch {
			case td && bd:
				b.WriteString("█")
			case td && !bd:
				b.WriteString("▀")
			case !td && bd:
				b.WriteString("▄")
			default:
				b.WriteByte(' ')
			}
		}
		b.WriteByte('\n')
	}
	fmt.Fprint(w, b.String())
}
