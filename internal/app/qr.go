package app

import (
	"io"
	"strings"

	"rsc.io/qr"
)

// Branded terminal QR (matches the menu-bar / phone-app pairing QR): the gtmux
// pane-grid mark is knocked out in the CENTER of the code on a light quiet patch,
// the same brand treatment Pairing.qrImage draws in the app. We encode at the
// HIGH error-correction level (qr.H, ~30% recovery) so the center knockout still
// scans, build the module matrix ourselves, overlay the mark, then render it as
// half-blocks (one terminal row = two QR rows).
//
// Terminal rendering note: a DARK module is drawn as empty space (the dark
// terminal bg shows through) and a LIGHT module as a lit block (█), so the code
// reads as dark-on-light to a scanner — the same polarity qrterminal uses.

const (
	qrLit   = "█" // both rows light  (= QR "white" module)
	qrDark  = " " // both rows dark   (= QR "black" module)
	qrUpper = "▀" // top light, bottom dark
	qrLower = "▄" // top dark, bottom light
	qrQuiet = 4   // quiet-zone modules around the code (scanner needs ≥4)
)

// printBrandQR renders payload as a half-block QR with the centered brand mark.
// On any encode failure it falls back to a plain (markless) half-block render so
// pairing never breaks.
func printBrandQR(w io.Writer, payload string) {
	code, err := qr.Encode(payload, qr.H)
	if err != nil {
		renderPlainQR(w, payload)
		return
	}
	n := code.Size
	grid := make([][]bool, n)
	for y := 0; y < n; y++ {
		grid[y] = make([]bool, n)
		for x := 0; x < n; x++ {
			grid[y][x] = code.Black(x, y) // true = dark module
		}
	}
	overlayBrandMark(grid)
	renderHalfBlocks(w, grid)
}

// renderPlainQR is the no-mark fallback (low EC, like the previous behaviour).
func renderPlainQR(w io.Writer, payload string) {
	code, err := qr.Encode(payload, qr.L)
	if err != nil {
		return
	}
	n := code.Size
	grid := make([][]bool, n)
	for y := 0; y < n; y++ {
		grid[y] = make([]bool, n)
		for x := 0; x < n; x++ {
			grid[y][x] = code.Black(x, y)
		}
	}
	renderHalfBlocks(w, grid)
}

// brandMark is the gtmux pane grid as a 7×7 bitmap (true = ink): two top cells
// (3×3 each, 1-col gap) over a full-width bottom bar — the same silhouette as the
// menu-bar status item and the app QR logo.
func brandMark() [7][7]bool {
	var m [7][7]bool
	for y := 0; y < 7; y++ {
		for x := 0; x < 7; x++ {
			topLeft := y < 3 && x < 3
			topRight := y < 3 && x >= 4
			bottomBar := y >= 4
			m[y][x] = topLeft || topRight || bottomBar
		}
	}
	return m
}

// overlayBrandMark knocks out a centered light patch and draws the brand mark in
// it. The patch is small relative to the code (a ~9-unit square) so qr.H's 30%
// recovery comfortably absorbs the damage.
func overlayBrandMark(g [][]bool) {
	n := len(g)
	unit := 1
	if n >= 70 {
		unit = 2 // scale the mark up on larger codes so it stays visible
	}
	const cols, rows, margin = 7, 7, 1
	kw := (cols + 2*margin) * unit
	kh := (rows + 2*margin) * unit
	x0 := (n - kw) / 2
	y0 := (n - kh) / 2

	set := func(x, y int, v bool) {
		if y >= 0 && y < n && x >= 0 && x < n {
			g[y][x] = v
		}
	}
	// 1) knock the whole patch to light (quiet ring + mark background).
	for y := y0; y < y0+kh; y++ {
		for x := x0; x < x0+kw; x++ {
			set(x, y, false)
		}
	}
	// 2) draw the mark (dark) inside the quiet margin.
	mark := brandMark()
	mx0, my0 := x0+margin*unit, y0+margin*unit
	for my := 0; my < rows; my++ {
		for mx := 0; mx < cols; mx++ {
			if !mark[my][mx] {
				continue
			}
			for dy := 0; dy < unit; dy++ {
				for dx := 0; dx < unit; dx++ {
					set(mx0+mx*unit+dx, my0+my*unit+dy, true)
				}
			}
		}
	}
}

// renderHalfBlocks prints the module grid as half-block characters with a quiet
// zone. Each output row packs two QR rows (curr = top, next = bottom).
func renderHalfBlocks(w io.Writer, g [][]bool) {
	n := len(g)
	dark := func(x, y int) bool {
		if y < 0 || y >= n || x < 0 || x >= n {
			return false // quiet zone is light
		}
		return g[y][x]
	}
	var b strings.Builder
	for y := -qrQuiet; y < n+qrQuiet; y += 2 {
		for x := -qrQuiet; x < n+qrQuiet; x++ {
			c, nx := dark(x, y), dark(x, y+1)
			switch {
			case c && nx:
				b.WriteString(qrDark)
			case c && !nx:
				b.WriteString(qrLower)
			case !c && !nx:
				b.WriteString(qrLit)
			default:
				b.WriteString(qrUpper)
			}
		}
		b.WriteByte('\n')
	}
	io.WriteString(w, b.String())
}
