package app

import (
	"github.com/chenchaoyi/gtmux/internal/radar"
	"strings"
	"testing"
)

func TestPaneTreeMarksOnlyVerifiedHQ(t *testing.T) {
	out := captureStdout(t, func() {
		printPaneTree([]radar.PaneRow{
			{PaneID: "%1", Loc: "hq:0.0", Session: "hq", Window: "0", Tier: "agent", Agent: "Codex", Role: "supervisor"},
			{PaneID: "%2", Loc: "HQ:0.0", Session: "HQ", Window: "0", Tier: "plain", Command: "bash"},
		})
	})
	if strings.Count(out, "[HQ]") != 1 || !strings.Contains(out, "Gtmux HQ [HQ] · Codex") || !strings.Contains(out, "%1") || !strings.Contains(out, "%2") {
		t.Fatalf("missing identity or altered targeting: %s", out)
	}
}
