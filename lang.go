package main

import (
	"fmt"
	"os"
	"strings"
)

// lang is the resolved output language: "en" (default) or "zh".
// Set from $GTMUX_LANG, then overridden by a global --lang=en|zh flag.
var lang = "en"

// tr picks the English or Chinese variant of a string.
func tr(en, zh string) string {
	if lang == "zh" {
		return zh
	}
	return en
}

// say prints a localized line to stdout.
func say(en, zh string) { fmt.Println(tr(en, zh)) }

// sae prints a localized line to stderr.
func sae(en, zh string) { fmt.Fprintln(os.Stderr, tr(en, zh)) }

// ANSI styling (matches the bash version's palette).
const (
	cBold   = "\033[1m"
	cDim    = "\033[2m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cCyan   = "\033[36m"
	cReset  = "\033[0m"
)

// pl pluralizes a tmux-jargon noun: "1 window" / "3 windows" in en; no plural in zh.
func pl(n int, noun string) string {
	if lang == "zh" || n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// dispWidth is the terminal display width of s, counting CJK/wide runes as 2.
// (Go's %-Ns pads by rune count and printf by bytes — both misalign wide chars.)
func dispWidth(s string) int {
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

// padRight left-aligns s in a field of at least width display columns.
func padRight(s string, width int) string {
	if pad := width - dispWidth(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}
