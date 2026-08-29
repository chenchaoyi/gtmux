package hq

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
)

// box renders a capture the draft reader can parse, holding draft.
func box(draft string) string {
	return "some transcript\n╭──────────────────╮\n│ ❯ " + draft + "   │\n╰──────────────────╯"
}

type spyPane struct {
	screen string
	inMode bool
	pasted []string
	enters int
}

func (s *spyPane) io() dispatch.IO {
	return dispatch.IO{
		CaptureColor: func() string { return s.screen },
		InMode:       func() bool { return s.inMode },
		Paste:        func(t string) error { s.pasted = append(s.pasted, t); return nil },
		Enter:        func() error { s.enters++; return nil },
		ExitMode:     func() error { return nil },
		Sleep:        func() {},
	}
}

// The incident: a rotation typed `/clear` into a box holding a half-written `%11 `, and
// the Enter submitted `%11 /clear` — which HQ read as an instruction and carried out on
// a live session. A rotation must never be the thing that submits someone's sentence.
func TestRotationWillNotTypeOverADraft(t *testing.T) {
	for _, draft := range []string{"%11 ", "%11 /clear", "half a thought"} {
		p := &spyPane{screen: box(draft)}
		input, ok, held := rotateHQ("%6", p.io())
		if ok || input != "" {
			t.Errorf("draft %q: rotated anyway (input=%q)", draft, input)
		}
		if len(p.pasted) != 0 || p.enters != 0 {
			t.Errorf("draft %q: typed into the box — pasted=%v enters=%d", draft, p.pasted, p.enters)
		}
		if held == "" {
			t.Errorf("draft %q: withheld silently — the caller has nothing to tell the user", draft)
		}
	}
}

// Copy/view-mode swallows keys as navigation, and a capture with no locatable input box
// is a screen we cannot read. Both are answered "not empty" — never typing is the safe
// side of every question this guard cannot settle.
func TestRotationHoldsWhenTheBoxCannotBeRead(t *testing.T) {
	cases := map[string]*spyPane{
		"copy-mode":    {screen: box(""), inMode: true},
		"no input box": {screen: "just scrollback, no box at all\nnothing here"},
	}
	for name, p := range cases {
		if _, ok, _ := rotateHQ("%6", p.io()); ok {
			t.Errorf("%s: rotated on a screen it could not read", name)
		}
		if len(p.pasted) != 0 || p.enters != 0 {
			t.Errorf("%s: typed anyway", name)
		}
	}
}

// And it still rotates when the box is genuinely empty — a guard that never lets the
// act through is not a fix, it is a different outage.
func TestRotationProceedsOnAnEmptyBox(t *testing.T) {
	p := &spyPane{screen: box("")}
	_, ok, held := rotateHQ("%6", p.io())
	if !ok {
		t.Fatalf("held on an empty box: %q", held)
	}
	if len(p.pasted) != 1 || !strings.HasPrefix(p.pasted[0], "/") {
		t.Errorf("pasted %v, want the agent's own reset command", p.pasted)
	}
	if p.enters != 1 {
		t.Errorf("enters=%d, want exactly one submit", p.enters)
	}
}
