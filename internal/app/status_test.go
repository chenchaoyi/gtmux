package app

import (
	"strings"
	"testing"
)

func panesWith(statuses ...string) []agentPane {
	var p []agentPane
	for _, s := range statuses {
		p = append(p, agentPane{status: s})
	}
	return p
}

func TestStatusLine(t *testing.T) {
	// plain: only non-zero, priority order waiting→working→idle, running omitted.
	got := statusLine(panesWith("waiting", "working", "working", "idle", "running"), true)
	if got != "‖1 ●2 ✓1" {
		t.Fatalf("plain = %q", got)
	}

	// empty when nothing waiting/working/idle (running-only → empty segment).
	if got := statusLine(panesWith("running"), true); got != "" {
		t.Fatalf("running-only should be empty, got %q", got)
	}
	if got := statusLine(nil, true); got != "" {
		t.Fatalf("no panes should be empty, got %q", got)
	}

	// colored: tmux markup with the authoritative hex + a trailing reset.
	c := statusLine(panesWith("waiting", "idle"), false)
	if !strings.Contains(c, "#[fg=#EF4444]‖1") || !strings.Contains(c, "#[fg=#22C55E]✓1") {
		t.Fatalf("colored missing markup: %q", c)
	}
	if !strings.HasSuffix(c, "#[default]") {
		t.Fatalf("colored must end with reset: %q", c)
	}
}
