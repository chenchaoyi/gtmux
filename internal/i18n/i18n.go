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
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Cyan   = "\033[36m"
	Reset  = "\033[0m"
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
