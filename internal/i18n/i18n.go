// Package i18n holds gtmux's output language state and the localized-print,
// pluralization, and display-width helpers shared by every command.
package i18n

import (
	"fmt"
	"os"
	"strings"
)

// lang is the resolved output language: "en" (default) or "zh".
// Set from $GTMUX_LANG, then overridden by a global --lang=en|zh flag.
var lang = "en"

// SetLang sets the output language. Unknown values are ignored.
func SetLang(l string) {
	if l == "zh" || l == "en" {
		lang = l
	}
}

// Lang returns the current output language ("en" or "zh").
func Lang() string { return lang }

// Tr picks the English or Chinese variant of a string.
func Tr(en, zh string) string {
	if lang == "zh" {
		return zh
	}
	return en
}

// Say prints a localized line to stdout.
func Say(en, zh string) { fmt.Println(Tr(en, zh)) }

// Sae prints a localized line to stderr.
func Sae(en, zh string) { fmt.Fprintln(os.Stderr, Tr(en, zh)) }

// ANSI styling (matches the bash version's palette).
const (
	Bold   = "\033[1m"
	Dim    = "\033[2m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Cyan   = "\033[36m"
	// Amber (256-color ~#F59E0B) is a MODIFIER color, not a state color — used only
	// for the errored-idle ⚠ marker so it can't be mistaken for waiting (red/yellow).
	Amber = "\033[38;5;214m"
	Reset = "\033[0m"
)

// Pl pluralizes a tmux-jargon noun: "1 window" / "3 windows" in en; no plural in zh.
func Pl(n int, noun string) string {
	if lang == "zh" || n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// DispWidth is the terminal display width of s, counting CJK/wide runes as 2.
// (Go's %-Ns pads by rune count and printf by bytes — both misalign wide chars.)
func DispWidth(s string) int {
	w := 0
	for _, r := range s {
		switch {
		case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
			r >= 0x2E80 && r <= 0x303E, // CJK radicals … punctuation
			r >= 0x3041 && r <= 0x33FF, // Hiragana … CJK symbols
			r >= 0x3400 && r <= 0x4DBF, // CJK Ext A
			r >= 0x4E00 && r <= 0x9FFF, // CJK Unified
			r >= 0xA000 && r <= 0xA4CF, // Yi
			r >= 0xAC00 && r <= 0xD7A3, // Hangul syllables
			r >= 0xF900 && r <= 0xFAFF, // CJK compat
			r >= 0xFE30 && r <= 0xFE4F, // CJK compat forms
			r >= 0xFF00 && r <= 0xFF60, // fullwidth forms
			r >= 0xFFE0 && r <= 0xFFE6,
			r >= 0x20000 && r <= 0x3FFFD:
			w += 2
		default:
			w++
		}
	}
	return w
}

// PadRight left-aligns s in a field of at least width display columns.
func PadRight(s string, width int) string {
	if pad := width - DispWidth(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// PadLeft right-aligns s in a field of at least width display columns —
// PadRight's mirror, for right-aligned columns like relative time or %.
func PadLeft(s string, width int) string {
	if pad := width - DispWidth(s); pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}

// TruncDisp truncates s to at most width display columns, "…"-suffixed when
// cut (CJK-aware — snip()/rune-count truncation misjudges width for wide runes).
func TruncDisp(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if DispWidth(s) <= width {
		return s
	}
	r := []rune(s)
	w, i := 0, 0
	for ; i < len(r); i++ {
		cw := DispWidth(string(r[i]))
		if w+cw > width-1 {
			break
		}
		w += cw
	}
	return strings.TrimRight(string(r[:i]), " ") + "…"
}

// ColorEnabled reports whether ANSI styling should be emitted: stdout is a terminal and
// NO_COLOR is unset. Colour is an ADDITION for a human reading a live screen — a pipe,
// a file, or a `--json` consumer must get the same bytes it always got, and NO_COLOR is
// the standing convention for opting out even on a terminal.
func ColorEnabled() bool {
	if _, off := os.LookupEnv("NO_COLOR"); off {
		return false
	}
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// WrapDisp breaks s into lines of at most width display columns. English breaks at
// spaces; Chinese has none, so a wide rune is its own break opportunity — the same
// rule a terminal or a browser uses, and the reason this cannot be strings.Fields.
// A word longer than the whole width goes on its own line rather than being cut.
func WrapDisp(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		lines = append(lines, wrapPara(para, width)...)
	}
	return lines
}

func wrapPara(s string, width int) []string {
	var lines []string
	var cur []string // the tokens on the line being built
	lw := 0
	flush := func() {
		lines = append(lines, strings.TrimRight(strings.Join(cur, ""), " "))
		cur, lw = nil, 0
	}
	for _, tok := range dispTokens(s) {
		tw := DispWidth(tok)
		if tok == " " && lw == 0 {
			continue // a line never opens with a space
		}
		if lw > 0 && lw+tw > width {
			if noLineStart(tok) {
				// Chinese never opens a line with closing punctuation, and hanging it
				// past the margin only moves the problem to the terminal's own wrap.
				// Take the character before it down to the next line instead.
				carry := popTail(&cur)
				flush()
				cur = append(cur, carry...)
				for _, c := range carry {
					lw += DispWidth(c)
				}
			} else {
				flush()
			}
		}
		cur = append(cur, tok)
		lw += tw
	}
	if len(cur) > 0 || len(lines) == 0 {
		flush()
	}
	return lines
}

// popTail removes the last printing token from a line so a closer can take it along.
// A line of one token stays put: there is nothing to carry, and an empty line is worse
// than two columns of overhang.
func popTail(cur *[]string) []string {
	toks := *cur
	i := len(toks) - 1
	for i >= 0 && toks[i] == " " {
		i--
	}
	if i <= 0 {
		return nil
	}
	carry := toks[i:]
	*cur = toks[:i]
	return carry
}

// noLineStart reports the closing punctuation Chinese typesetting never opens a
// line with.
func noLineStart(tok string) bool {
	// Ideographic comma and full stop, fullwidth comma, colon, semicolon, bang,
	// question mark, closing paren, and the three closing quote brackets.
	const closers = "\u3001\u3002\uff0c\uff1a\uff1b\uff01\uff1f\uff09\u300b\u300d\u300f"
	r := []rune(tok)
	return len(r) == 1 && strings.ContainsRune(closers, r[0])
}

// dispTokens splits into the pieces a line may break between: each space, each wide
// rune, and each run of narrow non-space runes.
func dispTokens(s string) []string {
	var out []string
	var word strings.Builder
	closeWord := func() {
		if word.Len() > 0 {
			out = append(out, word.String())
			word.Reset()
		}
	}
	for _, r := range s {
		switch {
		case r == ' ':
			closeWord()
			out = append(out, " ")
		case DispWidth(string(r)) == 2:
			closeWord()
			out = append(out, string(r))
		default:
			word.WriteRune(r)
		}
	}
	closeWord()
	return out
}
