package app

import (
	"fmt"
	"io"
	"strings"

	"rsc.io/qr"
)

// Branded terminal QR (matches the phone-app / menu-bar pairing QR): the gtmux
// 2×2 brand mark (top-left cyan, other three grey) sits on a white patch in the
// CENTER, the same treatment Pairing.drawPaneGrid draws in the app. Two things
// make it look right in a terminal:
//
//   - QUADRANT blocks (▘▝▀▖▌… one char = a 2×2 module cell) instead of half
//     blocks, so the code is ~half as wide AND half as tall — a compact, square
//     QR that fits the window instead of overflowing it.
//   - The brand mark is drawn in real color (ANSI truecolor) on a white patch,
//     so it reads like the app's logo, not a monochrome knockout.
//
// Polarity: a DARK module renders as empty (terminal bg shows through), a LIGHT
// module / quiet zone as a filled quadrant in the default fg — so it scans the
// same way qrterminal renders. Encoded at qr.H (~30% recovery) so the center
// logo occlusion still decodes.

// quadrant glyphs indexed by a 4-bit mask: top-left=1, top-right=2,
// bottom-left=4, bottom-right=8 (bit set = that sub-cell is filled).
var quadGlyph = [16]string{
	" ", "▘", "▝", "▀", "▖", "▌", "▞", "▛",
	"▗", "▚", "▐", "▜", "▄", "▙", "▟", "█",
}

// module cell kinds.
const (
	cDark  int8 = iota // empty (terminal bg)
	cLight             // filled, default fg (data + quiet zone)
	cWhite             // logo patch background
	cCyan              // logo accent cell
	cGrey              // logo neutral cells
)

// truecolor escapes for the logo kinds (cLight uses the terminal default).
var cellColor = map[int8]string{
	cWhite: "\x1b[38;2;255;255;255m",
	cCyan:  "\x1b[38;2;6;182;212m",   // Theme.Status.working (#06B6D4)
	cGrey:  "\x1b[38;2;142;142;147m", // systemGray (#8E8E93)
}

const qrReset = "\x1b[0m"

// printBrandQR renders payload as a quadrant-block QR with the centered, colored
// brand mark. On any encode failure it falls back to a markless render.
func printBrandQR(w io.Writer, payload string) {
	code, err := qr.Encode(payload, qr.H)
	if err != nil {
		renderPlainQR(w, payload)
		return
	}
	renderQuadrant(w, buildGrid(code, true))
}

func renderPlainQR(w io.Writer, payload string) {
	code, err := qr.Encode(payload, qr.L)
	if err != nil {
		return
	}
	renderQuadrant(w, buildGrid(code, false))
}

// buildGrid turns a QR code into a padded module grid (quiet zone + even size for
// quadrant packing) of cell kinds, optionally overlaying the brand mark.
func buildGrid(code *qr.Code, logo bool) [][]int8 {
	const qz = 4 // quiet-zone modules each side
	n := code.Size
	size := n + 2*qz
	if size%2 != 0 {
		size++ // pad to even so 2×2 quadrant packing is clean
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
	if logo {
		overlayBrandMark(g)
	}
	return g
}

// overlayBrandMark stamps the 2×2 brand mark (white patch, cyan top-left, grey
// rest) in the center. All offsets are even and aligned to the quadrant grid so
// every logo character is a single solid color. The patch is small (~22% wide,
// ~5% area) so qr.H's recovery absorbs it.
func overlayBrandMark(g [][]int8) {
	size := len(g)
	const cell, gap, margin = 2, 2, 2
	mark := 2*cell + gap            // 6
	knock := mark + 2*margin        // 10
	x0 := ((size - knock) / 2) &^ 1 // force even
	y0 := ((size - knock) / 2) &^ 1

	fill := func(x, y, w, h int, v int8) {
		for j := y; j < y+h; j++ {
			for i := x; i < x+w; i++ {
				if j >= 0 && j < size && i >= 0 && i < size {
					g[j][i] = v
				}
			}
		}
	}
	fill(x0, y0, knock, knock, cWhite) // white patch
	cx0, cx1 := x0+margin, x0+margin+cell+gap
	cy0, cy1 := y0+margin, y0+margin+cell+gap
	fill(cx0, cy0, cell, cell, cCyan) // top-left accent
	fill(cx1, cy0, cell, cell, cGrey)
	fill(cx0, cy1, cell, cell, cGrey)
	fill(cx1, cy1, cell, cell, cGrey)
}

// renderQuadrant prints the grid with one character per 2×2 module block.
func renderQuadrant(w io.Writer, g [][]int8) {
	size := len(g)
	var b strings.Builder
	for y := 0; y < size; y += 2 {
		for x := 0; x < size; x += 2 {
			tl, tr := g[y][x], g[y][x+1]
			bl, br := g[y+1][x], g[y+1][x+1]
			// when all four sub-cells share a logo color, emit a solid colored
			// block; otherwise a default-fg quadrant glyph from the fill mask.
			if c := cellColor[tl]; c != "" && tl == tr && tl == bl && tl == br {
				b.WriteString(c)
				b.WriteString("█")
				b.WriteString(qrReset)
				continue
			}
			mask := 0
			if tl != cDark {
				mask |= 1
			}
			if tr != cDark {
				mask |= 2
			}
			if bl != cDark {
				mask |= 4
			}
			if br != cDark {
				mask |= 8
			}
			b.WriteString(quadGlyph[mask])
		}
		b.WriteByte('\n')
	}
	fmt.Fprint(w, b.String())
}
