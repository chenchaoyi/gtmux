package resume

import (
	"reflect"
	"testing"
)

func TestCommandWords(t *testing.T) {
	for _, tc := range []struct {
		line string
		want []string
	}{
		{`'/synthetic tools/gtmux' codex -- resume 'thread'`, []string{"/synthetic tools/gtmux", "codex", "--", "resume", "thread"}},
		{`'/synthetic'\''s tools/gtmux' codex`, []string{"/synthetic's tools/gtmux", "codex"}},
		{`gtmux codex -- -C /synthetic\ work`, []string{"gtmux", "codex", "--", "-C", "/synthetic work"}},
		{`codex ""`, []string{"codex", ""}},
		{`codex Bob's prompt`, []string{"codex", "Bob's", "prompt"}},
		{`codex trailing\`, []string{"codex", `trailing\`}},
	} {
		if got := commandWords(tc.line); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: got %q, want %q", tc.line, got, tc.want)
		}
	}
}
