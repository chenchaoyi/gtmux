package radar

import "testing"

// Claude Code animated a BRAILLE spinner in its title through 2.1.227 and switched to
// HALF-CIRCLES in 2.1.228. gtmux only knew braille, so on an updated agent the frame
// leaked into the task and the radar read "◐ multipilot-companion feature dev".
//
// The lesson is not "add ◐". It is that the alphabet changes without notice, which is
// why the display path no longer depends on knowing it.
func TestSpinnerAlphabetsAreRecognised(t *testing.T) {
	for _, r := range []rune{'⠁', '⠂', '⠄', '⠈', '⠐'} { // braille, ≤2.1.227
		if !isSpinnerGlyph(r) {
			t.Errorf("braille %q must read as a spinner", r)
		}
	}
	for _, r := range []rune{'◐', '◑', '◒', '◓'} { // half-circles, ≥2.1.228
		if !isSpinnerGlyph(r) {
			t.Errorf("half-circle %q must read as a spinner", r)
		}
	}
	for _, r := range []rune{'a', '中', '1', '✳'} { // ✳ is the IDLE mark, not a spinner
		if isSpinnerGlyph(r) {
			t.Errorf("%q must not read as a spinner", r)
		}
	}
}

// A working pane must be reported as working whichever alphabet its agent animates —
// this is what a hook-less agent's status depends on.
func TestClassifyWorkingOnEitherSpinner(t *testing.T) {
	for _, title := range []string{"⠐ build the thing", "◐ build the thing"} {
		isAgent, _, status, task := classifyAgent(title, "2.1.229", nil)
		if !isAgent || status != "working" {
			t.Errorf("%q: isAgent=%v status=%q, want working", title, isAgent, status)
		}
		if task != "build the thing" {
			t.Errorf("%q: task=%q, want the spinner stripped", title, task)
		}
	}
}

// The half that survives the next alphabet: a leading symbol + space is decoration, so an
// unknown future spinner cannot reach the UI even though gtmux cannot name it.
func TestUnknownDecorationNeverReachesTheTask(t *testing.T) {
	_, _, _, task := classifyAgent("✻ refactor auth middleware", "2.1.229", nil)
	if task != "refactor auth middleware" {
		t.Errorf("task=%q, want the unknown glyph stripped", task)
	}
	// A real task keeps its text — tolerance must not become truncation.
	_, _, _, task = classifyAgent("⠐ 服务端需求 review 与实现评估", "2.1.229", nil)
	if task != "服务端需求 review 与实现评估" {
		t.Errorf("task=%q", task)
	}
	// No decoration, nothing removed — including a first word that merely looks odd.
	_, _, _, task = classifyAgent("⠐ 3D pipeline", "2.1.229", nil)
	if task != "3D pipeline" {
		t.Errorf("task=%q", task)
	}
}
