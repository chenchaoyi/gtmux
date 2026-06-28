package app

import (
	"fmt"
	"io"
	"strings"

	"rsc.io/qr"
)

// Branded terminal QR (matches the phone-app / menu-bar pairing QR): the gtmux
// brand mark sits on a white patch in the CENTER — the same motif as the app's
// BrandMark (MOBILE §1): two square top cells (top-left neutral, TOP-RIGHT cyan =
// the focused/waiting pane) over one wide bottom cell that spans both columns.
//
// Rendered with QUADRANT blocks (one char = 2 modules wide × 2 modules tall), so
// the whole code is HALF as many columns as the module count — compact enough to
// fit comfortably in a terminal. (Half-blocks render each module visually square
// but take a full column per module, which made the code too wide.) Phone QR
// scanners rectify the modest cell aspect via the finder patterns, so it scans
// fine. The brand mark is drawn in real color (ANSI truecolor) on a white patch,
// so it reads like the app's logo; its cells are even-aligned to land on whole
// quadrant characters.
//
// Polarity: a DARK module renders as empty (terminal bg shows through), a LIGHT
// module / quiet zone as a filled block in the default fg — so it scans the same
// way qrterminal renders. Encoded at qr.M (~15% recovery), enough for the small
// center logo while keeping the code smaller than level H.

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

// printBrandQR renders payload as a half-block QR with the centered, colored
// brand mark. On any encode failure it falls back to a markless render.
func printBrandQR(w io.Writer, payload string) {
	code, err := qr.Encode(payload, qr.M)
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

// buildGrid turns a QR code into a padded module grid (quiet zone + even row count
// for half-block packing) of cell kinds, optionally overlaying the brand mark.
func buildGrid(code *qr.Code, logo bool) [][]int8 {
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
	if logo {
		overlayBrandMark(g)
	}
	return g
}

// overlayBrandMark stamps the gtmux brand mark on a white patch in the center,
// matching the app's BrandMark: top-left neutral + top-right cyan (the focused
// pane), over a wide bottom cell spanning both columns. Offsets are even-aligned
// so each logo cell fills whole half-block characters (top module == bottom
// module → one solid colored block). The patch is small (~5% area) so qr.M's
// recovery absorbs it.
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
	fill(cx0, cy0, cell, cell, cGrey)       // top-left (neutral)
	fill(cx1, cy0, cell, cell, cCyan)       // top-right (cyan = focused/waiting pane)
	fill(cx0, cy1, 2*cell+gap, cell, cGrey) // bottom spans both columns
}

// quadrantGlyph maps a 4-bit on/off mask of the 2×2 module block to its Unicode
// quadrant block. Bit order: TL=8, TR=4, BL=2, BR=1 ("on" = a filled module).
var quadrantGlyph = [16]rune{
	' ', '▗', '▖', '▄', '▝', '▐', '▞', '▟',
	'▘', '▚', '▌', '▙', '▀', '▜', '▛', '█',
}

// renderQuadrant prints the grid with one character per 2-module-wide,
// 2-module-tall block — half the columns of a half-block render. The grid size is
// even (buildGrid pads it), so every 2×2 block is in range. A logo cell (a 2×2
// block of one color, even-aligned by overlayBrandMark) becomes a solid colored
// block; data blocks pick a quadrant glyph from their on/off pattern.
func renderQuadrant(w io.Writer, g [][]int8) {
	size := len(g)
	var b strings.Builder
	for y := 0; y < size; y += 2 {
		for x := 0; x < size; x += 2 {
			tl, tr := g[y][x], g[y][x+1]
			bl, br := g[y+1][x], g[y+1][x+1]
			// a uniform colored 2×2 logo cell → one solid colored block.
			if c := cellColor[tl]; c != "" && tl == tr && tl == bl && tl == br {
				b.WriteString(c)
				b.WriteRune('█')
				b.WriteString(qrReset)
				continue
			}
			mask := 0
			if tl != cDark {
				mask |= 8
			}
			if tr != cDark {
				mask |= 4
			}
			if bl != cDark {
				mask |= 2
			}
			if br != cDark {
				mask |= 1
			}
			b.WriteRune(quadrantGlyph[mask])
		}
		b.WriteByte('\n')
	}
	fmt.Fprint(w, b.String())
}
