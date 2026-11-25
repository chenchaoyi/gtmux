package app

import (
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/resume"
)

func TestDBR004DoctorDoesNotSuggestCodexForAReusedPane(t *testing.T) {
	former, _, _ := doctorAssociationFixture(t)
	former.Agent = "Claude Code"
	current := radar.PaneRow{Tier: "agent", Agent: "Codex", PaneID: "%10", Loc: "synthetic:0.1"}
	noVerifiedCodex := func(string, resume.Record) bool { return false }
	for _, tc := range []struct {
		name  string
		panes []radar.PaneRow
		guide bool
	}{
		{"only-reused-pane", []radar.PaneRow{former}, false},
		{"another-current-codex", []radar.PaneRow{former, current}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := staleBindingsFor(tc.panes, time.Now().Unix(), noVerifiedCodex)
			if row.status != stMiss || !strings.Contains(row.note, "%9 (proof-expired)") {
				t.Fatalf("the retained evidence was changed: %+v", row)
			}
			if strings.Contains(row.note, "gtmux focus %9") {
				t.Fatalf("the Claude pane received Codex instructions: %s", row.note)
			}
			for _, tip := range []string{"gtmux focus %10", "gtmux codex -- resume", "CODEX_HOME"} {
				if strings.Contains(row.note, tip) != tc.guide {
					t.Fatalf("wrong scope for %s: %s", tip, row.note)
				}
			}
		})
	}
}
