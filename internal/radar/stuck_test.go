package radar

import "testing"

func TestClassifyStuck(t *testing.T) {
	esc := "\x1b"
	box := func(draft string) string {
		return "history\n╭──────────────────────────────╮\n│ ❯ " + draft + " │\n╰──────────────────────────────╯"
	}
	gate := "Do you trust the files in this folder?\n" + box("")
	realDraft := box("please finish the migration")
	ghost := box(esc + "[2mping %14 to coordinate the charter" + esc + "[0m") // faint suggestion
	empty := box("")

	cases := []struct {
		name    string
		color   string
		preTurn bool
		want    string
	}{
		{"not a pre-turn dispatch → empty", realDraft, false, ""},
		{"startup gate", gate, true, "startup"},
		{"real unsubmitted draft", realDraft, true, "draft"},
		{"faint ghost is not a draft", ghost, true, ""},
		{"empty composer", empty, true, ""},
	}
	for _, c := range cases {
		if got := classifyStuck(c.color, "", c.preTurn); got != c.want {
			t.Errorf("%s: classifyStuck = %q, want %q", c.name, got, c.want)
		}
	}
}

// A gate phrase the pane is QUOTING rather than DRAWING must not read as a gate. This is
// the C20 defect in its exact production shape: a worker editing this repo's own gate
// table rendered `"": {"Do you trust the files"}` into its pane, and every slow tick
// after that read the scrollback as a live trust prompt — 36 `waiting·startup` knocks at
// HQ over 13.6 hours, from a pane that was working the whole time.
func TestClassifyStuck_QuotedGatePhraseInScrollback(t *testing.T) {
	var b []byte
	b = append(b, "the worker's own diff of internal/prompt/prompt.go:\n"...)
	b = append(b, "  var startupGates = map[string][]string{\n"...)
	b = append(b, "  \t\"\": {\"Do you trust the files\"}, // Claude trust-folder gate\n"...)
	b = append(b, "  }\n"...)
	// …then a screenful of ordinary work below it, and an idle composer at the bottom.
	for i := 0; i < 30; i++ {
		b = append(b, "  ok\n"...)
	}
	b = append(b, "╭──────────────────────────────╮\n│ ❯                            │\n╰──────────────────────────────╯"...)

	if got := classifyStuck(string(b), "", true); got != "" {
		t.Fatalf("a quoted gate phrase up in scrollback must not read as a gate; got %q", got)
	}
}
