// Package prompt parses an interactive coding-agent menu (Claude Code's
// "❯ 1. Yes / 2. … / 3. …" choice block) out of captured pane text. This is the
// ONE canonical parser (HANDOFF: shared by the menu-bar in-place reply, macOS
// notifications, and the mobile approval card) — surfaces consume it via
// `gtmux options <pane>` (CLI) or the serve API, never re-implement it.
package prompt

import (
	"regexp"
	"strings"
)

// Option is one numbered choice: N is the key the user presses (1/2/3…), Label is
// the agent's own wording for it.
type Option struct {
	N     int    `json:"n"`
	Label string `json:"label"`
}

// numbered matches a cleaned line like "1. Yes, proceed" → (1, "Yes, proceed").
var numbered = regexp.MustCompile(`^(\d+)\.\s+(.*\S)`)

// ansiCSI matches an ANSI CSI escape (e.g. the SGR color codes "\x1b[38;5;153m",
// "\x1b[0m") and ansiOSC an OSC sequence (e.g. the "\x1b]8;;file://…" hyperlinks
// Claude Code emits), terminated by BEL or ST. The ESC byte is non-printing, so
// when a colored capture leaks these into an option line only the "[…m" / "]…"
// tail shows — we strip them so labels are plain text. (See clean.)
var (
	ansiCSI = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")
	ansiOSC = regexp.MustCompile("\x1b\\][^\x07\x1b]*(?:\x07|\x1b\\\\)")
)

// ParseOptions extracts the LAST contiguous run of options starting at 1 from the
// captured pane text (the most recent menu on screen). Returns nil when there's
// no parseable choice block — callers fall back to "jump to reply".
//
// It tolerates the box-drawing chrome Claude Code draws around the menu (leading
// │ ╭ ╰ ─ and the ❯/> selector, trailing │ and padding) and resets whenever it
// sees a fresh "1." so a stale earlier menu never wins.
func ParseOptions(text string) []Option {
	var opts []Option
	want := 1
	for _, raw := range strings.Split(text, "\n") {
		line := clean(raw)
		m := numbered.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n := 0
		for _, c := range m[1] {
			n = n*10 + int(c-'0')
		}
		label := strings.TrimSpace(m[2])
		if n == 1 {
			// a new menu started — restart the run.
			opts = []Option{{N: 1, Label: label}}
			want = 2
			continue
		}
		if n == want {
			opts = append(opts, Option{N: n, Label: label})
			want++
		}
	}
	if len(opts) == 0 {
		return nil
	}
	return opts
}

// selectorGlyphs are the cursor marks interactive TUI choice menus put on the
// highlighted row (Claude ❯, Codex ›, others ▶ ▸ →). A numbered list in prose
// output has none — so requiring one tells an ACTIVE approval menu apart from a
// list, which is what lets us detect "waiting" from the screen for agents that
// (unlike Claude) fire no waiting hook.
const selectorGlyphs = "❯›▶▸→"

// WaitingOptions returns the on-screen choice block ONLY when it looks like an
// ACTIVE approval menu the agent is blocked on: a run of ≥2 numbered options in
// the bottom of the capture, with a selector cursor present. It's deliberately
// stricter than ParseOptions (which callers use once a pane is already known to
// be waiting) so it can be used to DETECT waiting without false-positiving on a
// numbered list. Returns nil otherwise.
func WaitingOptions(text string) []Option {
	lines := strings.Split(text, "\n")
	end := len(lines)
	for end > 0 && clean(lines[end-1]) == "" {
		end-- // ignore trailing blank / chrome-only lines
	}
	if end == 0 {
		return nil
	}
	lo := end - 14 // only the bottom of the screen — the active prompt lives there
	if lo < 0 {
		lo = 0
	}
	window := lines[lo:end]
	opts := ParseOptions(strings.Join(window, "\n"))
	if len(opts) < 2 {
		return nil // a real menu has ≥2 choices; a lone "1." is likely a list item
	}
	for _, l := range window {
		if strings.ContainsAny(l, selectorGlyphs) {
			return opts // an interactive selector is present → a live menu
		}
	}
	return nil
}

// clean strips the menu's box-drawing/selector chrome so numbered() can match the
// content: ANSI color/hyperlink escapes anywhere in the line, then the leading
// │ ╭ ╰ ╮ ╯ ─ space and the ❯ / > selector, and a trailing │ + padding spaces.
func clean(s string) string {
	s = ansiOSC.ReplaceAllString(s, "")
	s = ansiCSI.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "│╭╰╮╯─ \t")
	s = strings.TrimLeft(s, selectorGlyphs+"> \t") // ❯ (Claude) › ▶ ▸ → (others) > and padding
	s = strings.TrimRight(s, "│ \t")
	return s
}
