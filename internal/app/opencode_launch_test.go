package app

import (
	"reflect"
	"testing"
)

func TestOpenCodeStandaloneCompatibility(t *testing.T) {
	for _, tc := range []struct {
		version    string
		args, want []string
	}{
		{"opencode v2.2.5\n", []string{"--session", "ses_one"}, []string{"--standalone", "--session", "ses_one"}},
		{"1.18.11", []string{"--session", "ses_one"}, []string{"--session", "ses_one"}},
		{"unknown", []string{}, []string{}},
		{"3.0.0", []string{"--session", "ses_one"}, []string{"--session", "ses_one"}},
		{"2.2.5", []string{"--server", "http://localhost:123"}, []string{"--server", "http://localhost:123"}},
		{"2.2.5", []string{"run", "hello"}, []string{"run", "hello"}},
		{"2.2.5", []string{"--standalone"}, []string{"--standalone"}},
		{"2.2.5", []string{"mcp", "list"}, []string{"mcp", "list"}},
		{"2.2.5", []string{"--standalone=false"}, []string{"--standalone=false"}},
		{"2.2.5", []string{"--standalone=true"}, []string{"--standalone=true"}},
		{"2.2.5", []string{"--no-standalone"}, []string{"--no-standalone"}},
		{"2.2.5", []string{"--no-standalone=false"}, []string{"--no-standalone=false"}},
	} {
		if got := opencodeStandaloneArgs(tc.args, tc.version); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s %v: %v", tc.version, tc.args, got)
		}
	}
}
