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

	"github.com/chenchaoyi/gtmux/internal/ansi"
)

// DraftOf returns the input-box draft text of a full-screen capture and whether a
// structured input region (a box or recognized prompt) was located. It reuses the
// #393 region detector — the same primitive Deliver's screen-read fallback uses — so
// the HQ-nudge draft-guard shares ONE definition of "what is a draft". `structured`
// is false when no input box is locatable (a plain shell, a full-screen view): the
// caller then cannot confirm the box is empty and MUST NOT type into it.
func DraftOf(capture string) (draft string, structured bool) {
	_, draft, structured = SplitInputRegion(capture)
	return draft, structured
}

// DraftOfColored is the faint-aware DraftOf: given a COLOR capture (`capture-pane -e`),
// it EXCLUDES any text the agent renders FAINT (SGR 2) before locating the draft — that
// is Claude Code's suggested-next-command GHOST text (a dim autosuggestion that needs a
// key to accept), which is NOT user input. On a plain CaptureFull the faint markers are
// stripped, so the ghost is indistinguishable from a typed draft and every "is there an
// unsubmitted draft?" caller false-positives (a stuck `waiting`, a suppressed `done`, a
// held HQ nudge). Real user input is normal brightness and a pasted goal is bright too,
// so dropping faint spans can only remove a ghost, never real input. Callers that match
// a SPECIFIC payload (Deliver's verify, the wake-ack `#id`) don't need this — a ghost
// can never equal their target — and keep using plain SplitInputRegion.
func DraftOfColored(coloredCapture string) (draft string, structured bool) {
	_, draft, structured = SplitInputRegion(ansi.StripDroppingFaint(coloredCapture))
	return draft, structured
}

// SplitInputRegion divides a full-screen capture into the HISTORY region (the
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
//
// Exported because the HQ wake channel's ack needs the same distinction: a batch id
// found in the DRAFT means the paste landed but the Enter did not — the opposite of
// delivered.
func SplitInputRegion(capture string) (history, draft string, structured bool) {
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
		// A single border, two shapes:
		//   - a left-edge box CLOSED by a bottom rule (opencode: "┃  text" lines above
		//     a "╹▀▀▀" border) — the draft is the ┃/│-prefixed block ABOVE the border;
		//   - a lone TOP rule over the input — the draft is BELOW it.
		if edge := leftEdgeBlockStart(lines, bottom); edge >= 0 {
			return strings.Join(lines[:edge], "\n"), stripBoxChrome(lines[edge:bottom]), true
		}
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

// labeledBorderMinRun is the shortest CONTIGUOUS run of horizontal rule characters that
// makes a line carrying label text still a border. A titled box rule (Claude Code 2.x
// titles the input box, e.g. "──────── SAT ──" or "──── Context left …: 41% ──") always
// spans much of the width, so its rule run is long; a real content line has at most a
// stray dash or two scattered among its text, never an 8-long run. 8 sits far below any
// real border and far above any content line.
const labeledBorderMinRun = 8

// isBoxBorder reports whether a line is a horizontal box-drawing border (the top/bottom
// of an input box): a horizontal box-drawing RUN (with corners, pipes, and spacing),
// long enough not to be a stray single glyph. A pure content line ("│ > hello │") has no
// rule and is NOT a border. A TITLED rule ("──── SAT ──") IS a border — the label is
// tolerated only when a long contiguous rule run is present, because a mis-recognized
// titled top border was splitting the draft into the status footer and failing every
// send into a Claude Code 2.x pane ("the input box didn't confirm the full message").
func isBoxBorder(line string) bool {
	t := strings.TrimSpace(line)
	if utf8.RuneCountInString(t) < 3 {
		return false
	}
	horiz, label, run, maxRun := 0, 0, 0, 0
	for _, r := range t {
		switch r {
		case '─', '━', '═', '╌', '╍', '┄', '┅', '┈', '┉',
			// half-block rules some TUIs draw the input box with (opencode's
			// bottom border is "╹▀▀▀▀…"). NOT the full block '█', which the
			// opencode/ASCII banners are made of — those must stay non-borders.
			'▀', '▄', '▔', '▁':
			horiz++
			run++
			if run > maxRun {
				maxRun = run
			}
		case '╭', '╮', '╰', '╯', '┌', '┐', '└', '┘', '├', '┤', '┬', '┴', '┼',
			'│', '┃', '║', '╹', '╻', '╺', '╸', ' ', '\t':
			// allowed border furniture, but not itself a "horizontal run"
			run = 0
		default:
			label++ // embedded label text (a titled box rule, e.g. "──── SAT ──")
			run = 0
		}
	}
	if horiz < 2 { // require a real horizontal run, not just corners/pipes
		return false
	}
	// A pure rule / furniture line is a border (the original behavior). A line that ALSO
	// carries label text is a border only if it has a long CONTIGUOUS rule run — the
	// title of a box, not a content line that happens to hold a dash or two.
	return label == 0 || maxRun >= labeledBorderMinRun
}

// leftEdgeBlockStart returns the first index of the consecutive block of left-edge
// box lines ("┃ …" / "│ …") immediately above the border at `bottom`, or -1 when the
// line directly above the border is not a left-edge line (so the border is a plain
// top rule, not the bottom of a left-edge box). This locates opencode's composer,
// whose input box is drawn with a left ┃ edge and a ╹▀▀▀ bottom rule but NO top rule.
func leftEdgeBlockStart(lines []string, bottom int) int {
	isLeftEdge := func(s string) bool {
		t := strings.TrimSpace(s)
		return strings.HasPrefix(t, "┃") || strings.HasPrefix(t, "│")
	}
	if bottom-1 < 0 || !isLeftEdge(lines[bottom-1]) {
		return -1
	}
	start := bottom - 1
	for start-1 >= 0 && isLeftEdge(lines[start-1]) {
		start--
	}
	return start
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
