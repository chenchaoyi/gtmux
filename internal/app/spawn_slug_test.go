package app

import "testing"

func TestSlugify(t *testing.T) {
	for _, c := range [][2]string{
		{"Widen the popover", "widen-the-popover"},
		{"feat/menubar-width", "feat-menubar-width"},
		{"  spaces  and---dashes  ", "spaces-and-dashes"},
		{"UPPER_snake.case", "upper-snake-case"},
		{"emoji ✳ goal 123", "emoji-goal-123"},
		{"", ""},
		{"！！！", ""},
		{"a-really-long-title-that-exceeds-the-cap-limit", "a-really-long-title-that"},
	} {
		if got := slugify(c[0]); got != c[1] {
			t.Errorf("slugify(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

// spawnSlug prefers --title, then the branch leaf, then a normalized goal head.
func TestSpawnSlug(t *testing.T) {
	if got := spawnSlug("My Title", "feat/x", "some goal"); got != "my-title" {
		t.Errorf("explicit title wins: %q", got)
	}
	if got := spawnSlug("", "feat/menubar-width", "widen the popover"); got != "menubar-width" {
		t.Errorf("branch leaf: %q", got)
	}
	if got := spawnSlug("", "", "widen the popover to 420 pixels wide"); got != "widen-the-popover-to" {
		t.Errorf("goal head (first 4 words): %q", got)
	}
	if got := spawnSlug("", "", ""); got != "" {
		t.Errorf("nothing → empty: %q", got)
	}
}
