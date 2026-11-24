package hqinput

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestBoundBriefRequiresDurableResetCompletion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	b := RotationBrief{Request: "original", Pane: "%6", Agent: "codex", Home: "home", Session: "successor",
		Stamp: "10:20:30", Instance: "frontend", ClientPID: 21, Deadline: 103 + int64(RotationReadyWait/time.Second)}
	r := map[string]any{"pane": b.Pane, "agent": b.Agent, "retiring": "old", "input": "/new", "createdAt": 100,
		"sentAt": 101, "switchedAt": 103, "bootstrapAt": 104, "successor": b.Session, "targetStamp": b.Stamp,
		"bootstrapID": b.Request, "bootstrapHome": b.Home, "bootstrapInstance": b.Instance, "bootstrapPID": b.ClientPID}
	for _, tc := range []struct {
		name   string
		second int64
		valid  bool
	}{
		{"legacy-binding", 0, false}, {"before-attempt", 100, false}, {"completed", 102, true}, {"after-briefing", 105, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.second > 0 {
				r["submittedAtNS"] = time.Unix(tc.second, 0).UnixNano()
			}
			raw, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if err := state.WritePrivateFile(filepath.Join(state.Dir(), "hqwake", "rotate-pending.json"), raw); err != nil {
				t.Fatal(err)
			}
			if got := MatchesRotationBrief(b); got != tc.valid {
				t.Fatalf("bound request authorized=%v, want %v", got, tc.valid)
			}
			if window, err := RotationWindow(b.Pane, 104); err != nil || window != RotationHandoff {
				t.Fatalf("bound input reservation changed: %v, %v", window, err)
			}
		})
	}
}
