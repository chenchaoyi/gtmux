package dispatch

import (
	"strings"
	"unicode"
)

// The one answer to "does the box hold this payload and nothing else?". It decides when an
// Enter may be pressed on a box someone else could have typed into: the wake channel's
// Enter-only repair (hqnudge) and a delivery's swallowed-Enter retry both ask it, so a
// rule learned on one (text typed before or after ours must never be submitted with it,
// #1294 review R3) holds on the other.

// DraftIsExactly reports whether the input box holds payload and nothing else, as
// this package's draft extraction returns it (one string per displayed row, joined by
// newlines). It is the extraction's inverse, and accepts exactly two differences:
//
//   - whitespace, removed from both sides: the TUI re-wraps the line, indents its
//     continuation rows and pads the box;
//   - the glyphs the extraction strips from each row's edges as box chrome — a border
//     (│ ┃) at either end and one prompt marker (❯ › > ▌) at the start — may be missing
//     from the draft where a row began or ended. A wake line contains them too (its
//     " │ " separator), so one lands on a row edge whenever the wrap puts it there, and
//     is stripped with the chrome.
//
// Anything else refuses: text added, removed or changed anywhere, and a glyph added or
// removed inside a row. What it cannot see, no comparison can: a glyph the user typed at
// a row's edge is stripped before the draft reaches us.
func DraftIsExactly(draft, payload string) bool {
	p := []rune(withoutSpace(payload))
	if len(p) == 0 {
		return false
	}
	var rows [][]rune
	for _, ln := range strings.Split(draft, "\n") {
		rows = append(rows, []rune(withoutSpace(ln)))
	}
	type state struct{ row, at int }
	seen := map[state]bool{}
	// match reports whether rows[row:] account for p[at:] exactly.
	var match func(row, at int) bool
	match = func(row, at int) bool {
		if row == len(rows) {
			return at == len(p)
		}
		if seen[state{row, at}] {
			return false
		}
		seen[state{row, at}] = true
		for _, a := range edgeSkips(p, at, "│") { // the extraction trims │ then ┃, each once
			for _, b := range edgeSkips(p, a, "┃") {
				for _, c := range edgeSkips(p, b, "❯", "›", ">", "▌") {
					end := c + len(rows[row])
					if end > len(p) || string(p[c:end]) != string(rows[row]) {
						continue
					}
					for _, d := range edgeSkips(p, end, "┃") {
						for _, e := range edgeSkips(p, d, "│") {
							if match(row+1, e) {
								return true
							}
						}
					}
				}
			}
		}
		return false
	}
	return match(0, 0)
}

// edgeSkips returns the positions a row-edge strip may resume from at p[at]: at itself,
// and at+1 when p[at] is one of the glyphs the extraction strips there.
func edgeSkips(p []rune, at int, glyphs ...string) []int {
	out := []int{at}
	if at < len(p) {
		for _, g := range glyphs {
			if string(p[at]) == g {
				return append(out, at+1)
			}
		}
	}
	return out
}

func withoutSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}
