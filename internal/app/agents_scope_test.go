package app

import "testing"

func TestAgentsPaneFilterRejectsInvalidOrNonJSONRequests(t *testing.T) {
	for _, args := range [][]string{{"--pane"}, {"--pane", "%7"}, {"--json", "--pane", "plain"}, {"--json", "--pane", "%7;bad"}, {"--json", "--pane", "%"}, {"--json", "--pane", "%7", "--watch"}, {"--json", "--pane", "%7", "--popup"}} {
		if got := cmdAgents(args); got != 1 {
			t.Fatalf("%v: status %d", args, got)
		}
	}
}
