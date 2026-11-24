package resume

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
)

func TestStrictFrontendReadinessClassifiesMissingAndChangedProof(t *testing.T) {
	for _, mode := range []string{"fresh", "expired", "missing-record", "index-mismatch", "changed-bridge", "retired", "wrong-home", "missing-pane", "changed-pane"} {
		t.Run(mode, func(t *testing.T) {
			f, _ := frontendFixture(t)
			f, err := BindCodexFrontend(f, "root-a")
			if err != nil {
				t.Fatal(err)
			}
			view := frontendPaneView
			frontendPaneView = func(pane, format string) string {
				if format == "#{session_name}:#{window_index}.#{pane_index}" {
					return f.Loc
				}
				if mode == "missing-pane" {
					return ""
				}
				if mode == "changed-pane" {
					return "%7\t900\tchanged:0.0"
				}
				return view(pane, format)
			}
			wantCode, retry := "", false
			switch mode {
			case "missing-pane":
				wantCode, retry = "pane-proof-unavailable", true
			case "changed-pane":
				wantCode = "pane-changed"
			case "expired":
				f.StartedAt -= 20000
				f.UpdatedAt = time.Now().Add(-10 * time.Second).UnixMilli()
				if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
					t.Fatal(err)
				}
				wantCode, retry = "proof-expired", true
			case "missing-record":
				if err := os.Remove(frontendPath(f.Instance)); err != nil {
					t.Fatal(err)
				}
				wantCode, retry = "frontend-record-unavailable", true
			case "index-mismatch":
				r, _ := Load(f.Loc)
				r.SessionID = "replaced"
				if err := writeBindingJSON(fileFor(f.Loc), r); err != nil {
					t.Fatal(err)
				}
				wantCode = "frontend-index-mismatch"
			case "retired", "wrong-home":
				stored := f
				wantCode = "frontend-retired"
				if mode == "retired" {
					stored.Retired = true
				} else {
					stored.HomeID = strings.Repeat("a", 32)
					wantCode = "frontend-index-mismatch"
				}
				if err := writeBindingJSON(frontendPath(f.Instance), stored); err != nil {
					t.Fatal(err)
				}
			case "changed-bridge":
				process := frontendProcess
				frontendProcess = func(pid int) (launchhost.AgentProcess, bool) {
					p, ok := process(pid)
					if pid == f.BridgePID {
						p.Start = "changed"
					}
					return p, ok
				}
				wantCode = "bridge-changed"
			}
			found, err := CodexFrontendForPaneReadiness(f.Pane)
			if _, live := CodexFrontendForPane(f.Pane); live != (wantCode == "") || found.Instance != f.Instance ||
				(found.HomeID != f.HomeID) != (mode == "wrong-home") {
				t.Fatal("readiness changed authority or lost observed identity", live, err)
			}
			if wantCode == "" && err == nil {
				return
			}
			var why *FrontendBindingError
			if !errors.As(err, &why) || why.Code != wantCode || why.Retryable != retry {
				t.Fatalf("wrong readiness: %v, want %q retry=%v", err, wantCode, retry)
			}
		})
	}
}
