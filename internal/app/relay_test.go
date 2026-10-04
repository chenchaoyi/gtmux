package app

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/relay"
)

func TestRelayAttentionAndAttribution(t *testing.T) {
	report := relay.Request{ID: "report-1", Kind: "report", Pane: "%7", Session: "build", Blocking: false}
	if report.Blocking {
		t.Fatal("ordinary report should remain ledger-only")
	}
	ask := relay.Request{ID: "ask-1", Kind: "ask", Pane: "%7", Session: "build", TaskID: "t42", Blocking: true, Reply: "Check the build log"}
	wake := relayWake(ask)
	if hqwake.PriorityOf(wake) != hqwake.PriorityDecision || hqwake.GradeOf(hqwake.ClassAgentRelay) != hqwake.GradeDecision {
		t.Fatalf("blocking ask was not prominent: %q", wake)
	}
	if strings.Contains(wake, ask.Reply) || !strings.Contains(wake, "request:ask-1") {
		t.Fatalf("wake contains body or lost ID: %q", wake)
	}
	reply := relayReplyText(ask)
	for _, want := range []string{"HQ reply", "ask-1", "build (%7)", "t42", "not a user instruction or authorization"} {
		if !strings.Contains(reply, want) {
			t.Errorf("reply missing %q: %q", want, reply)
		}
	}
}

func TestRelayReplyStaysWithOriginalPaneLifetime(t *testing.T) {
	r := relay.Request{Pane: "%7", Session: "build", PanePID: "123"}
	if !relaySourceMatches(r, "%7", "build", "123") {
		t.Fatal("live source rejected")
	}
	for _, v := range [][3]string{{"%8", "build", "123"}, {"%7", "other", "123"}, {"%7", "build", "456"}} {
		if relaySourceMatches(r, v[0], v[1], v[2]) {
			t.Fatalf("reused source accepted: %v", v)
		}
	}
}

func TestSpawnMentionsRelayAndCodexPermission(t *testing.T) {
	if !strings.Contains(relayContext, "gtmux relay report|ask") || !strings.Contains(relayContext, "not user authorization") {
		t.Fatal("agent context omitted relay boundary")
	}
	for _, tc := range []struct{ agent, want string }{{"codex", "codex --approve-for-me"}, {"codex -a on-request", "codex -a on-request"}, {"codex --sandbox read-only", "codex --sandbox read-only"}, {"codex -a never", "codex -a never"}, {"claude", "claude"}} {
		if got := agentLaunchCommand(tc.agent, ""); got != tc.want {
			t.Errorf("launch %q = %q, want %q", tc.agent, got, tc.want)
		}
	}
}
