package ghostty

import "testing"

// TestSessionsFromTitles: extract the session (before " — ") from each tab
// title, in order, de-duped, skipping non-matching lines (and iTerm's " (tmux)").
func TestSessionsFromTitles(t *testing.T) {
	in := "Diting — Diting Dev\nPica — sat (tmux)\nplain bash prompt\nDiting — Diting Dev\n日常更新 — kb\n"
	got := SessionsFromTitles(in)
	want := []string{"Diting", "Pica", "日常更新"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%q want %q", i, got[i], want[i])
		}
	}
}
