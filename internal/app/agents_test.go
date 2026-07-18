package app

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/radar"
)

func TestAgentsSummary(t *testing.T) {
	defer i18n.SetLang("en")
	i18n.SetLang("en")

	if got := agentsSummary(nil); got != "0 agents" {
		t.Errorf("empty summary = %q, want %q", got, "0 agents")
	}
	panes := []radar.Pane{
		{Status: "waiting"}, {Status: "working"}, {Status: "idle"}, {Status: "running"},
	}
	want := "4 agents · 1 waiting · 1 working · 2 idle" // running counts toward idle bucket
	if got := agentsSummary(panes); got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}
