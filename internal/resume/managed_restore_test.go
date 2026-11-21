package resume

import (
	"strings"
	"testing"
	"time"
)

func TestManagedRestoreConfirmationRequiresNewLiveFrontend(t *testing.T) {
	for _, kind := range []string{"exact", "standalone", "retired-index", "stale-index", "unverified", "retired-frontend",
		"old-frontend", "different-instance", "different-home", "different-thread", "different-pane",
		"different-location", "different-root", "reused-root", "different-tty", "different-directory",
		"different-thread-directory", "unknown-send"} {
		t.Run(kind, func(t *testing.T) {
			id, _ := cnrAttemptIdentity(t)
			a, err := ReserveNativeRestoreAttempt(id)
			if err != nil {
				t.Fatal(err)
			}
			result := "pending"
			if kind == "unknown-send" {
				result = "unknown"
			}
			a, err = UpdateNativeRestoreAttempt(a, result)
			if err != nil {
				t.Fatal(err)
			}
			seen := a.IssuedAt / int64(time.Second)
			other := t.TempDir()
			current := Record{Agent: "codex", HomeID: id.HomeID, SessionID: id.SessionID, Cwd: id.Cwd,
				UpdatedAt: seen, Frontend: strings.Repeat("a", 32)}
			f := CodexFrontend{Instance: current.Frontend, HomeID: id.HomeID, SessionID: id.SessionID,
				StartedAt: a.IssuedAt/int64(time.Millisecond) + 1, ThreadCwd: id.Cwd,
				CodexBindingTarget: CodexBindingTarget{Pane: id.Pane, Loc: id.Loc, PanePID: id.RootPID,
					RootStart: id.RootBirth, TTY: "/dev/" + id.TTY, Cwd: id.Cwd}}
			verified := true
			switch kind {
			case "standalone":
				current.Frontend = ""
			case "retired-index":
				current.FrontendRetired = true
			case "stale-index":
				current.UpdatedAt--
			case "unverified":
				verified = false
			case "retired-frontend":
				f.Retired = true
			case "old-frontend":
				f.StartedAt--
			case "different-instance":
				f.Instance = strings.Repeat("b", 32)
			case "different-home":
				f.HomeID = "different-home"
			case "different-thread":
				f.SessionID = "different-session"
			case "different-pane":
				f.Pane = "%2"
			case "different-location":
				f.Loc = "other:0.0"
			case "different-root":
				f.PanePID++
			case "reused-root":
				f.RootStart = "new-root-birth"
			case "different-tty":
				f.TTY = "/dev/ttys002"
			case "different-directory":
				f.Cwd = other
			case "different-thread-directory":
				f.ThreadCwd = other
			}
			old := restoreFrontendForPane
			t.Cleanup(func() { restoreFrontendForPane = old })
			restoreFrontendForPane = func(pane string) (CodexFrontend, bool) {
				if pane != id.Pane {
					t.Fatalf("looked up wrong pane %q", pane)
				}
				return f, verified
			}
			want := kind == "exact"
			if got := ConfirmManagedRestoreAttempt(a, current); got != want {
				t.Fatalf("confirmed=%v, expected %v", got, want)
			}
			if want {
				result = "confirmed"
			}
			onDisk, exists, err := readNativeRestoreAttempt(nativeRestoreAttemptPath(id))
			if err != nil || !exists || onDisk.State != result {
				t.Fatalf("attempt state=%q exists=%v err=%v, expected %q", onDisk.State, exists, err, result)
			}
		})
	}
}
