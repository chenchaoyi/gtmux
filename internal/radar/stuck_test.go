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
		name         string
		plain, color string
		tracked      bool
		want         string
	}{
		{"not tracked → empty", realDraft, realDraft, false, ""},
		{"startup gate", gate, gate, true, "startup"},
		{"real unsubmitted draft", realDraft, realDraft, true, "draft"},
		{"faint ghost is not a draft", empty, ghost, true, ""},
		{"empty composer", empty, empty, true, ""},
	}
	for _, c := range cases {
		if got := classifyStuck(c.plain, c.color, "", c.tracked); got != c.want {
			t.Errorf("%s: classifyStuck = %q, want %q", c.name, got, c.want)
		}
	}
}
