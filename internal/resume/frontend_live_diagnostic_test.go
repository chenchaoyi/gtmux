package resume

import (
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"os"
	"testing"
	"time"
)

func TestHRW013LiveDiagnosticsDoNotChangeExistingAuthority(t *testing.T) {
	for _, mode := range []string{"valid", "legacy", "missing-record", "missing-frontend", "expired", "missing-bridge", "changed-bridge", "changed-pane", "index-mismatch"} {
		t.Run(mode, func(t *testing.T) {
			f, _ := frontendFixture(t)
			var err error
			f, err = BindCodexFrontend(f, "root-a")
			if err != nil {
				t.Fatal(err)
			}
			r, ok := Load(f.Loc)
			if !ok {
				t.Fatal("fixture binding absent")
			}
			wantOK, wantCode := false, ""
			switch mode {
			case "valid":
				wantOK = true
			case "legacy":
				r.Frontend = ""
				if err := writeBindingJSON(fileFor(f.Loc), r); err != nil {
					t.Fatal(err)
				}
				wantOK = true
			case "missing-record":
				if err := os.Remove(fileFor(f.Loc)); err != nil {
					t.Fatal(err)
				}
				wantCode = "resume-record-unavailable"
			case "missing-frontend":
				if err := os.Remove(frontendPath(f.Instance)); err != nil {
					t.Fatal(err)
				}
				wantCode = "frontend-record-unavailable"
			case "expired":
				f.StartedAt -= 20000
				f.UpdatedAt = time.Now().Add(-10 * time.Second).UnixMilli()
				if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
					t.Fatal(err)
				}
				wantCode = "proof-expired"
			case "missing-bridge":
				old := frontendProcess
				frontendProcess = func(pid int) (launchhost.AgentProcess, bool) {
					if pid == f.BridgePID {
						return launchhost.AgentProcess{}, false
					}
					return old(pid)
				}
				wantCode = "bridge-proof-unavailable"
			case "changed-bridge":
				old := frontendProcess
				frontendProcess = func(pid int) (launchhost.AgentProcess, bool) {
					p, ok := old(pid)
					if pid == f.BridgePID {
						p.Start = "different-birth"
					}
					return p, ok
				}
				wantCode = "bridge-changed"
			case "changed-pane":
				frontendPaneView = func(string, string) string { return "%other\t100\tother:0.0" }
				wantCode = "pane-changed"
			case "index-mismatch":
				r.HomeID = "different-home"
				if err := writeBindingJSON(fileFor(f.Loc), r); err != nil {
					t.Fatal(err)
				}
				wantCode = "frontend-index-mismatch"
			}
			beforeRecord, _ := os.ReadFile(fileFor(f.Loc))
			beforeProof, _ := os.ReadFile(frontendPath(f.Instance))
			diagnostic, live, reason := LoadLiveDiagnostic(f.Loc)
			original, originalLive := LoadLive(f.Loc)
			if live != wantOK || reason != wantCode || originalLive != live || original != diagnostic {
				t.Fatalf("%s: live=%v code=%q legacy=%v want=%v/%q", mode, live, reason, originalLive, wantOK, wantCode)
			}
			afterRecord, _ := os.ReadFile(fileFor(f.Loc))
			afterProof, _ := os.ReadFile(frontendPath(f.Instance))
			if string(beforeRecord) != string(afterRecord) || string(beforeProof) != string(afterProof) {
				t.Fatal("diagnostic repaired/wrote private authority")
			}
		})
	}
}
