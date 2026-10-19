package prompt

import "testing"

func TestDSHCompactComposerReadiness(t *testing.T) {
	for _, tt := range []struct {
		s     string
		ready bool
	}{{"⌸ ❯ ", true}, {"⌸ ❯ INSERT draft", true}, {"⌸ ❯ NORMAL draft", false}, {"❯ 1. Choose a session", false}, {"Launchpad\n> Search", false}, {"Expanded editor\ntext\nINSERT 1:2", false}, {"Approve tool\n1. Yes", false}} {
		if got := IsComposerReady(tt.s, "dsh"); got != tt.ready {
			t.Fatalf("%q ready=%v", tt.s, got)
		}
	}
}
