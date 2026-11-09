package native

import (
	"syscall"
	"testing"
	"time"
)

func TestClosedFrontendObservationSurvivesPositivePIDReuseOnly(t *testing.T) {
	oldProbe, oldStart := probeClosedFrontend, procStart
	t.Cleanup(func() { probeClosedFrontend, procStart = oldProbe, oldStart })
	for _, name := range []string{"gone", "reused after binding", "original process", "within tolerance", "unknown start", "permission unknown", "future start", "new generation during start read"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			now := time.Now().Unix()
			r := Record{Agent: "codex", HomeID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SessionID: "worker", Cwd: "/synthetic", State: "working", Event: "Resumed", PID: 100, Comm: "codex", ProcessStart: "backend-birth", UpdatedAt: now - 400}
			if err := Save(r); err != nil {
				t.Fatal(err)
			}
			r, _ = Load(r.SessionID, r.HomeID)
			b := Binding{Kind: "frontend", HomeID: r.HomeID, ID: "binding", SessionID: r.SessionID, Cwd: r.Cwd, Pane: "%999", Loc: "synthetic:0.0", PID: 200, PanePID: 300, BackendPID: r.PID, BackendStart: r.ProcessStart, PreparedAt: now - 600, BoundAt: now - 300}
			if err := ConfirmBinding(b, r); err != nil {
				t.Fatal(err)
			}
			probeClosedFrontend = func(int) error {
				if name == "gone" {
					return syscall.ESRCH
				}
				if name == "permission unknown" {
					return syscall.EPERM
				}
				return nil
			}
			procStart = func(pid int) (int64, bool) {
				if pid != b.PID {
					t.Fatalf("wrong PID start read: %d", pid)
				}
				switch name {
				case "original process":
					return b.PreparedAt, true
				case "within tolerance":
					return b.BoundAt + startSlack, true
				case "unknown start":
					return 0, false
				case "future start":
					return now + 60, true
				case "new generation during start read":
					if err := Save(r); err != nil {
						t.Fatal(err)
					}
				}
				return b.BoundAt + startSlack + 1, true
			}
			want := name == "gone" || name == "reused after binding"
			if got := ClosedFrontendObservation(r); got != want {
				t.Fatalf("closed=%v want %v", got, want)
			}
			if _, ok := Load(r.SessionID, r.HomeID); !ok {
				t.Fatal("proof removed observation")
			}
		})
	}
}
