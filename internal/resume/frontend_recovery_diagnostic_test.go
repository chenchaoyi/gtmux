package resume

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/launchhost"
)

func TestDPC003RecoveryEvidenceNeverGrantsRetryAuthority(t *testing.T) {
	for _, mode := range []string{"absent", "unreadable-process", "reused-pid", "stale-live", "healthy", "missing-proof", "foreign-session"} {
		t.Run(mode, func(t *testing.T) {
			f, _ := frontendFixture(t)
			var err error
			f, err = BindCodexFrontend(f, "root-a")
			if err != nil {
				t.Fatal(err)
			}
			oldGone := frontendRecoveryGone
			t.Cleanup(func() { frontendRecoveryGone = oldGone })
			frontendRecoveryGone = func(int) bool { return mode == "absent" }
			want := FrontendRecoveryUnconfirmed
			switch mode {
			case "absent", "unreadable-process":
				frontendProcess = func(int) (launchhost.AgentProcess, bool) { return launchhost.AgentProcess{}, false }
				want = FrontendRecoveryUnknown
				if mode == "absent" {
					want = FrontendRecoveryStopped
				}
			case "reused-pid":
				frontendProcess = func(pid int) (launchhost.AgentProcess, bool) {
					return launchhost.AgentProcess{PID: pid, Start: "replacement"}, true
				}
				want = FrontendRecoveryStopped
			case "missing-proof":
				if err := os.Remove(frontendPath(f.Instance)); err != nil {
					t.Fatal(err)
				}
				want = FrontendRecoveryUnknown
			case "foreign-session":
				f.SessionID = "foreign"
				want = FrontendRecoveryUnknown
			}
			if mode != "missing-proof" {
				if mode != "healthy" {
					f.UpdatedAt = time.Now().Add(-time.Minute).UnixMilli()
				}
				if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(frontendPath(f.Instance))
			recordBefore, ok := Load(f.Loc)
			if !ok {
				t.Fatal("fixture lacks retained conversation")
			}
			if got := LoadFrontendRecoveryKind(f.Loc); got != want {
				t.Fatalf("%s: got %s want %s", mode, got, want)
			}
			after, _ := os.ReadFile(frontendPath(f.Instance))
			recordAfter, _ := Load(f.Loc)
			if !bytes.Equal(before, after) || recordBefore != recordAfter {
				t.Fatal("diagnostic changed binding evidence")
			}
		})
	}
}
