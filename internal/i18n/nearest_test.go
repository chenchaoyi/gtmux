package i18n

import (
	"strings"
	"testing"
)

func TestNearestCatchesTheMistakesPeopleMake(t *testing.T) {
	cmds := []string{"agents", "adopt", "attach", "awake", "usage", "update", "knowledge"}
	cases := []struct {
		typed string
		want  string // the first suggestion
	}{
		{"agent", "agents"},   // one letter short
		{"usag", "usage"},     // a truncated word
		{"kn", "knowledge"},   // a prefix
		{"updaet", "update"},  // two letters swapped
		{"attatch", "attach"}, // one letter too many
	}
	for _, c := range cases {
		got := Nearest(c.typed, cmds, 3)
		if len(got) == 0 || got[0] != c.want {
			t.Errorf("Nearest(%q) = %v, want %q first", c.typed, got, c.want)
		}
	}
}

func TestNearestStaysQuietWhenNothingIsClose(t *testing.T) {
	if got := Nearest("xyzzy", []string{"agents", "usage", "restore"}, 3); len(got) != 0 {
		t.Errorf("Nearest(\"xyzzy\") = %v, want nothing — a wrong guess is worse than none", got)
	}
	if got := Nearest("", []string{"agents"}, 3); len(got) != 0 {
		t.Errorf("Nearest(\"\") = %v, want nothing", got)
	}
}

func TestNearestReturnsAtMostWhatYouAskedFor(t *testing.T) {
	opts := []string{"list", "lint", "land", "link", "limit"}
	got := Nearest("lis", opts, 2)
	if len(got) > 2 {
		t.Errorf("Nearest asked for 2 returned %d: %v", len(got), got)
	}
	if len(got) == 0 || got[0] != "list" {
		t.Errorf("Nearest(%q) = %v, want the prefix match first", "lis", got)
	}
	// Stable: the same typo suggests the same thing every time.
	for i := 0; i < 5; i++ {
		if again := Nearest("lis", opts, 2); strings.Join(again, ",") != strings.Join(got, ",") {
			t.Fatalf("suggestion order changed between runs: %v then %v", got, again)
		}
	}
}
