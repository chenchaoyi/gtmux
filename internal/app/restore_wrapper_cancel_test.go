package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

// The private tmux stub announces that the final query has started, then waits
// for release before returning the original, successful shell identity.
func TestCompatibilityCancellationDuringFinalTargetQuery(t *testing.T) {
	for _, stop := range []bool{false, true} {
		name := "stable"
		if stop {
			name = "cancelled"
		}
		t.Run(name, func(t *testing.T) {
			a, b, calls := visibleRestoreFixture(t, false, nil)
			a.Launcher, b.Launcher = "fixture-wrapper", "fixture-wrapper"
			if err := os.WriteFile(calls, nil, 0600); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			t.Setenv("QUERY_DIR", dir)
			changeQueuedTmuxReply(t, "display-message) ", `display-message)
if [ "$4" = "%7" ]; then
  : > "$QUERY_DIR/started"
  n=0
  while [ ! -f "$QUERY_DIR/release" ]; do
    n=$((n+1))
    [ "$n" -lt 1000 ] || exit 1
    sleep 0.01
  done
fi
`)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			released := make(chan error, 1)
			go func() {
				for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
					if _, err := os.Stat(filepath.Join(dir, "started")); err == nil {
						if stop {
							cancel()
						}
						released <- os.WriteFile(filepath.Join(dir, "release"), nil, 0600)
						return
					}
				}
				released <- fmt.Errorf("final target query did not start")
			}()

			used := map[string]bool{}
			gate := newRestoreSerialGate(ctx, resumeAuto, 2)
			gate.observe = func(resumeAttempt) bool { return true }
			issue := func(pane, loc string, r resume.Record) (resumeAttempt, bool) {
				return issueCompatibleRestore(ctx, pane, loc, r, resumeAuto, used, nil)
			}
			var first resumeAttempt
			var sent bool
			gate.run(func() (resumeAttempt, bool) {
				first, sent = issue("%7", "pair:0.0", a)
				return first, sent
			})
			if err := <-released; err != nil {
				t.Fatal(err)
			}
			secondStarted := false
			gate.run(func() (resumeAttempt, bool) {
				secondStarted = true
				return issue("%8", "pair:0.1", b)
			})
			body, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			if stop {
				if first.State != "cancelled" || sent || secondStarted || len(used) != 0 || len(body) != 0 {
					t.Fatalf("cancelled: first=%q sent=%v second=%v claims=%d writes=%q", first.State, sent, secondStarted, len(used), body)
				}
				return
			}
			if first.State != "pending" || !sent || !secondStarted || !used[a.SessionID] || !used[b.SessionID] {
				t.Fatalf("stable: first=%q sent=%v second=%v claims=%d", first.State, sent, secondStarted, len(used))
			}
			for _, pane := range []string{"%7", "%8"} {
				if strings.Count(string(body), "send-keys -t "+pane+" -l") != 1 || strings.Count(string(body), "send-keys -t "+pane+" Enter") != 1 {
					t.Fatalf("stable %s writes: %q", pane, body)
				}
			}
		})
	}
}
