package agents

import "testing"

func TestOpenCodeInteractiveRecipes(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{nil, true},
		{[]string{"mini", "--session", "ses_one"}, true},
		{[]string{"mcp", "list"}, false},
		{[]string{"run", "hello"}, false},
		{[]string{"--server=http://localhost:123"}, false},
		{[]string{"--standalone=false"}, false},
		{[]string{"--no-standalone"}, false},
		{[]string{"--no-standalone=false"}, false},
		{[]string{"--standalone=true"}, true},
	} {
		if got := OpenCodeInteractiveArgs(tc.args); got != tc.want {
			t.Errorf("%v: got %v, want %v", tc.args, got, tc.want)
		}
	}
}
