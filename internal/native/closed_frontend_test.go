package native

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestClosedFrontendObservationRequiresExactSnapshotAndGoneProcess(t *testing.T) {
	old, oldStart := probeClosedFrontend, procStart
	t.Cleanup(func() { probeClosedFrontend, procStart = old, oldStart })
	for _, name := range []string{"exact closed", "alive", "unknown", "missing", "corrupt", "wrong home", "wrong thread", "legacy", "other backend", "other birth", "before observation", "future binding", "no generation", "terminal evidence", "new observation", "update during probe", "rebind during probe"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			now := time.Now().Unix()
			procStart = func(int) (int64, bool) { return now - 60, true }
			r := Record{Agent: "codex", HomeID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SessionID: "worker", Cwd: "/synthetic", State: "working", Event: "Resumed", PID: 100, Comm: "codex", ProcessStart: "backend-birth", UpdatedAt: now - 5}
			if name == "terminal evidence" {
				r.Terminal = "Synthetic Terminal"
			}
			if err := Save(r); err != nil {
				t.Fatal(err)
			}
			r, _ = Load(r.SessionID, r.HomeID)
			b := Binding{Kind: "frontend", HomeID: r.HomeID, ID: "binding", SessionID: r.SessionID, Cwd: r.Cwd, Pane: "%999", Loc: "synthetic:0.0", PID: 200, PanePID: 300, BackendPID: r.PID, BackendStart: r.ProcessStart, PreparedAt: now - 30, BoundAt: now - 1}
			switch name {
			case "wrong home":
				b.HomeID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			case "wrong thread":
				b.SessionID = "other-thread"
			case "legacy":
				b.Kind = ""
			case "other backend":
				b.BackendPID++
			case "other birth":
				b.BackendStart = "another-birth"
			case "before observation":
				b.BoundAt = now - 10
			case "future binding":
				b.BoundAt = now + 60
			}
			if err := ConfirmBinding(b, r); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "missing":
				if err := os.Remove(bindingPath(r.SessionID, r.HomeID)); err != nil {
					t.Fatal(err)
				}
			case "corrupt":
				if err := os.WriteFile(bindingPath(r.SessionID, r.HomeID), []byte(`{"invalid":true}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "new observation":
				if err := Save(r); err != nil { // same second and fields, new generation
					t.Fatal(err)
				}
			case "no generation":
				r.Revision = ""
			}
			probeClosedFrontend = func(pid int) error {
				if pid != b.PID {
					t.Fatalf("probed wrong process: %d", pid)
				}
				switch name {
				case "alive":
					return nil
				case "unknown":
					return syscall.EPERM
				case "update during probe":
					if err := Save(r); err != nil {
						t.Fatal(err)
					}
				case "rebind during probe":
					b.PanePID++
					if err := ConfirmBinding(b, r); err != nil {
						t.Fatal(err)
					}
				}
				return syscall.ESRCH
			}
			before, _ := os.ReadFile(recordPath(r))
			if got := ClosedFrontendObservation(r); got != (name == "exact closed") {
				t.Fatalf("proof=%v want %v", got, name == "exact closed")
			}
			after, err := os.ReadFile(recordPath(r))
			if err != nil {
				t.Fatal("proof removed observation", err)
			}
			if name != "update during probe" && string(before) != string(after) {
				t.Fatal("read changed observation bytes")
			}
		})
	}
}
