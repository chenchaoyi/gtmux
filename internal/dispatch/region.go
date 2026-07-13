// Package dispatch implements verified task delivery to a coding-agent pane —
// the machinery behind `gtmux spawn` and `gtmux send`'s default verify. Its
// centerpiece is Deliver: paste the task, then confirm it actually LANDED, with
// deterministic hook evidence preferred and a hardened screen-read as a fallback.
// All I/O is injected (see IO), so the whole state machine is unit-testable
// without tmux — the CLI stays cgo-free.
package dispatch

import (
	"strings"
	"unicode/utf8"
)

// DraftOf returns the input-box draft text of a full-screen capture and whether a
// structured input region (a box or recognized prompt) was located. It reuses the
// #393 region detector — the same primitive Deliver's screen-read fallback uses — so
// the HQ-nudge draft-guard shares ONE definition of "what is a draft". `structured`
// is false when no input box is locatable (a plain shell, a full-screen view): the
// caller then cannot confirm the box is empty and MUST NOT type into it.
func DraftOf(capture string) (draft string, structured bool) {
	_, draft, structured = splitInputRegion(capture)
	return draft, structured
}

// splitInputRegion divides a full-screen capture into the HISTORY region (the
// conversation transcript, above the input box) and the DRAFT region (what sits in
// the input box, not yet submitted). Most agent TUIs (Claude Code, Codex) draw a
// box around the input; its top/bottom borders are the structural separator, so
// "❯ text" inside the box is unambiguously a DRAFT — not something already
// submitted. This is the fix for the "is ❯ text draft or sent?" ambiguity.
//
// Location, not a fixed offset: we find the LAST box (its bottom border scanning up,
// then its top border) and take the lines between the borders as the draft. History
// is everything above the top border. When no box is present (a plain readline
// prompt), degrade to: draft = the tail from the last prompt marker.
//
// `structured` reports whether a real input region (box or recognized prompt) was
// found. When false, the capture has no locatable draft (e.g. a plain shell prompt),
// so a caller MUST NOT treat an empty draft as a fragment — there's nothing to
// destroy-and-retry; post-submit verification decides instead.
func splitInputRegion(capture string) (history, draft string, structured bool) {
	lines := strings.Split(capture, "\n")
	// Find the bottom border: the last box-border line in the capture.
	bottom := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if isBoxBorder(lines[i]) {
			bottom = i
			break
		}
	}
	if bottom < 0 {
		return splitByPrompt(lines)
	}
	// Find the top border: the next box-border above the bottom one.
	top := -1
	for i := bottom - 1; i >= 0; i-- {
		if isBoxBorder(lines[i]) {
			top = i
			break
		}
	}
	if top < 0 {
		// A single border (e.g. only a top rule over the input) — treat everything
		// below it as draft, above as history.
		return strings.Join(lines[:bottom], "\n"), stripBoxChrome(lines[bottom+1:]), true
	}
	history = strings.Join(lines[:top], "\n")
	draft = stripBoxChrome(lines[top+1 : bottom])
	return history, draft, true
}

// splitByPrompt is the no-box degrade path: the draft is the tail starting at the
// last prompt marker line ("❯ ", "> ", "│ > "), history is everything before it.
// structured is false when NO prompt marker is found (a plain shell), so the caller
// won't misread "no draft" as a fragment.
func splitByPrompt(lines []string) (history, draft string, structured bool) {
	mark := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if isPromptLine(lines[i]) {
			mark = i
			break
		}
	}
	if mark < 0 {
		// No structure at all — treat the whole capture as history (nothing is
		// confidently a draft), so a landing must show up in history to count.
		return strings.Join(lines, "\n"), "", false
	}
	return strings.Join(lines[:mark], "\n"), stripBoxChrome(lines[mark:]), true
}

// isBoxBorder reports whether a line is a horizontal box-drawing border (the
// top/bottom of an input box): made up solely of box-drawing runes, corners, and
// spacing, and long enough not to be a stray single glyph. A content line inside
// the box ("│ > hello │") contains letters, so it is NOT a border.
func isBoxBorder(line string) bool {
	t := strings.TrimSpace(line)
	if utf8.RuneCountInString(t) < 3 {
		return false
	}
	horiz := 0
	for _, r := range t {
		switch r {
		case '─', '━', '═', '╌', '╍', '┄', '┅', '┈', '┉':
			horiz++
		case '╭', '╮', '╰', '╯', '┌', '┐', '└', '┘', '├', '┤', '┬', '┴', '┼',
			'│', '┃', '║', ' ', '\t':
			// allowed border furniture, but not itself a "horizontal run"
		default:
			return false
		}
	}
	return horiz >= 2 // require a real horizontal run, not just corners/pipes
}

// isPromptLine reports whether a line looks like an input prompt (a leading ❯/›/>,
// optionally wrapped in a box pipe). Used only on the no-box degrade path.
func isPromptLine(line string) bool {
	t := strings.TrimSpace(line)
	t = strings.TrimPrefix(t, "│")
	t = strings.TrimPrefix(t, "┃")
	t = strings.TrimSpace(t)
	for _, p := range []string{"❯", "›", ">", "▌"} {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// stripBoxChrome removes leading/trailing box pipes and prompt markers from draft
// lines so the residual is the user's actual draft text, then joins them. It keeps
// the text content (for the head match) while dropping the box furniture.
func stripBoxChrome(lines []string) string {
	var out []string
	for _, ln := range lines {
		t := ln
		t = strings.TrimSpace(t)
		t = strings.TrimPrefix(t, "│")
		t = strings.TrimSuffix(t, "│")
		t = strings.TrimPrefix(t, "┃")
		t = strings.TrimSuffix(t, "┃")
		t = strings.TrimSpace(t)
		for _, p := range []string{"❯ ", "› ", "> ", "▌ ", "❯", "›", ">", "▌"} {
			if strings.HasPrefix(t, p) {
				t = strings.TrimSpace(strings.TrimPrefix(t, p))
				break
			}
		}
		out = append(out, t)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
