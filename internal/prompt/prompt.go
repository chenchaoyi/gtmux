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

// clean strips the menu's box-drawing/selector chrome so numbered() can match the
// content: leading │ ╭ ╰ ╮ ╯ ─ space and the ❯ / > selector, and a trailing │ +
// padding spaces.
func clean(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "│╭╰╮╯─ \t")
	s = strings.TrimLeft(s, "❯> \t")
	s = strings.TrimRight(s, "│ \t")
	return s
}
