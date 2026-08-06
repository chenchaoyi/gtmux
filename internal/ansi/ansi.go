// Package ansi strips ANSI terminal escapes from captured pane text — the one place
// that logic lives, instead of two independent implementations (a regex pair in
// `prompt` and a hand-rolled state machine in `dispatch`) that handled malformed tails
// and OSC termination differently and could drift.
package ansi

import "strings"

// Strip removes all ANSI escapes (CSI + OSC + two-byte ESC sequences), keeping the
// literal text — the plain-text form the menu parser and box detector work on.
func Strip(s string) string { return strip(s, false) }

// StripDroppingFaint removes ANSI escapes AND drops any literal text emitted while SGR
// FAINT (code 2) is active — Claude Code renders its suggested-next-command GHOST text
// faint, and on a plain capture that dim text is indistinguishable from a real draft.
// Box chrome and normal-brightness input (never faint) survive.
func StripDroppingFaint(s string) string { return strip(s, true) }

// strip is the shared one pass over the string:
//   - an SGR sequence (ESC[…m) updates the faint flag (2 → on; 0 or 22 → off) and emits
//     nothing;
//   - any other CSI (ESC[…<final>) or OSC (ESC]…BEL/ST) escape emits nothing;
//   - a literal rune is emitted — unless dropFaint is set and faint is currently on.
//
// It keys narrowly on SGR 2 (the confirmed ghost-text signal). Extended-color params —
// 256-color `38;5;n` and 24-bit truecolor `38;2;r;g;b` (and the `48`/`58` variants) —
// carry a `5`/`2` and RGB/index values that are NOT intensity codes, so applyFaint skips
// their sub-parameters (a truecolor's color-space id is always 2; a 256-color index can be
// 2/0/22). Without this, every truecolor line — a Codex composer + footer is truecolor —
// was read as faint and dropped, leaking the flag and swallowing newlines.
func strip(s string, dropFaint bool) string {
	var b strings.Builder
	faint := false
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] != 0x1b { // not ESC — a literal rune
			if !(dropFaint && faint) {
				b.WriteRune(rs[i])
			}
			continue
		}
		if i+1 >= len(rs) {
			break
		}
		switch rs[i+1] {
		case '[': // CSI: ESC[ params <final-byte in @..~>
			j := i + 2
			for j < len(rs) && !(rs[j] >= '@' && rs[j] <= '~') {
				j++
			}
			if j < len(rs) {
				if rs[j] == 'm' { // an SGR — update faint from its params
					faint = applyFaint(faint, string(rs[i+2:j]))
				}
				i = j // skip through the final byte
			} else {
				i = len(rs) // malformed tail
			}
		case ']': // OSC: ESC] … BEL or ST(ESC\)
			j := i + 2
			for j < len(rs) && rs[j] != 0x07 && !(rs[j] == 0x1b && j+1 < len(rs) && rs[j+1] == '\\') {
				j++
			}
			if j < len(rs) && rs[j] == 0x1b {
				j++ // consume the '\' of ST
			}
			i = j
		default: // a two-char ESC sequence (best-effort) — skip the following byte
			i++
		}
	}
	return b.String()
}

// applyFaint folds an SGR parameter list into the running faint flag: `2` turns faint
// on; `0` (reset all) or `22` (normal intensity) turns it off; an empty param list is a
// bare `ESC[m` = reset. The extended-color introducers 38/48/58 (fg/bg/underline color)
// are followed by sub-parameters — `5;n` (256-color index) or `2;r;g;b` (24-bit
// truecolor) — whose values must NOT be read as intensity: a truecolor's color-space id
// is always `2`, and a 256-color index can be 2/0/22. So skip an extended-color run's
// sub-parameters. Other params don't affect intensity.
func applyFaint(faint bool, params string) bool {
	if params == "" {
		return false // ESC[m == ESC[0m
	}
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		switch parts[i] {
		case "2":
			faint = true
		case "0", "22":
			faint = false
		case "38", "48", "58": // extended color — skip its sub-params (5;n or 2;r;g;b)
			if i+1 < len(parts) {
				switch parts[i+1] {
				case "5":
					i += 2 // "5" + the 256-color index
				case "2":
					i += 4 // "2" + r;g;b
				default:
					i++ // malformed — consume just the introducer's next token
				}
			}
		}
	}
	return faint
}
