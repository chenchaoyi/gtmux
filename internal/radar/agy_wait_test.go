package radar

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestAGYSEL002_AntigravityScreenWaitLifecycleAndDigest(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(state.ActiveDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(state.WaitingDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	now := time.Now().Unix()
	// PreInvocation wrote active/%20 when the Antigravity turn began.
	if err := state.WriteMarker(state.ActivePath("%20"), "agy-conv-1"); err != nil {
		t.Fatal(err)
	}

	menuScreen := "Run this command?\n" +
		"> 1. Yes, run command\n" +
		"  2. Yes, and always allow in this conversation for commands that start with 'git status'\n" +
		"  3. No, cancel\n\n" +
		"  ↑/↓ Navigate · tab Amend · esc to cancel\n"
	runningScreen := "✓ Ran git status\n• Running tests… (2s · esc to cancel)\n" +
		"────────────────────────────────────────\n" +
		">\n" +
		"────────────────────────────────────────\n"

	currentFrame := menuScreen
	origCapture := paneCapture
	paneCapture = func(id string) string {
		if id == "%20" {
			return currentFrame
		}
		return ""
	}
	defer func() { paneCapture = origCapture }()

	line := strings.Join([]string{
		"%20", "gtmux antigravity", "0", "0",
		"agy", "agy", "0", strconv.FormatInt(now, 10), "4242", t.TempDir(), "0", "", "1",
	}, "\t")

	withFixture(t, []string{line}, func() {
		// 1. Before slow-tick writes waiting/%20, radar + digest + PaneHasScreenAsk
		// immediately surface waiting, kind="permission", and replyable Ask options.
		panes := GatherAgents()
		if len(panes) != 1 || panes[0].Agent != "Antigravity" || panes[0].Status != "waiting" {
			t.Fatalf("GatherAgents on agy menu = %+v, want Antigravity waiting", panes)
		}
		if !PaneHasScreenAsk("%20") {
			t.Fatal("PaneHasScreenAsk(%20) = false, want true while live agy menu is on screen")
		}
		digest := GatherDigest()
		if len(digest) != 1 || digest[0].Status != "waiting" || digest[0].Kind != "permission" {
			t.Fatalf("GatherDigest on agy menu = %+v, want status=waiting kind=permission", digest)
		}
		if !strings.Contains(digest[0].Ask, "1.Yes, run command") || !strings.Contains(digest[0].Ask, "3.No, cancel") {
			t.Fatalf("GatherDigest Ask = %q, want parsed options", digest[0].Ask)
		}

		// 2. After slow-tick persists waiting/%20="permission" and the user approves
		// option 1, the menu leaves the screen while active/%20 remains: status must
		// immediately return to "working" even if the command runs quietly with 0% CPU.
		if err := state.WriteMarker(state.WaitingPath("%20"), "permission"); err != nil {
			t.Fatal(err)
		}
		currentFrame = runningScreen
		panes = GatherAgents()
		if len(panes) != 1 || panes[0].Status != "working" {
			t.Fatalf("GatherAgents after agy menu answered = %+v, want working", panes)
		}
		if PaneHasScreenAsk("%20") {
			t.Fatal("PaneHasScreenAsk(%20) = true after menu cleared, want false")
		}

		// 3. On a settled Antigravity pane (first observation = no frame-change burst)
		// with no active turn and no menu on screen, a lingering screen-permission
		// marker is removed and radar reports idle.
		idleLine := strings.Join([]string{
			"%21", "gtmux antigravity idle", "0", "0",
			"agy", "agy", "0", strconv.FormatInt(now-120, 10), "4243", t.TempDir(), "0", "", "1",
		}, "\t")
		if err := state.WriteMarker(state.WaitingPath("%21"), "permission"); err != nil {
			t.Fatal(err)
		}
		origCapture2 := paneCapture
		paneCapture = func(id string) string { return runningScreen }
		defer func() { paneCapture = origCapture2 }()
		withFixture(t, []string{idleLine}, func() {
			idlePanes := GatherAgents()
			if len(idlePanes) != 1 || idlePanes[0].Status != "idle" {
				t.Fatalf("GatherAgents on settled idle agy pane = %+v, want idle", idlePanes)
			}
			if state.Exists(state.WaitingPath("%21")) {
				t.Fatal("expected stale screen-permission marker on idle agy pane to be removed")
			}
		})
	})
}
