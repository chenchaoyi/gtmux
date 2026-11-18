package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func changeQueuedTmuxReply(t *testing.T, before, after string) {
	t.Helper()
	path := filepath.Join(os.Getenv("HOME"), "tmux")
	if tmux.Bin != path {
		t.Fatal("lost private sender")
	}
	data, err := os.ReadFile(path)
	if err != nil || strings.Count(string(data), before) != 1 {
		t.Fatalf("cannot locate private fixture reply: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), before, after, 1)), 0700); err != nil {
		t.Fatal(err)
	}
}

func observeQueuedRestore(t *testing.T, calls string, change func(), wantSecond bool) {
	t.Helper()
	send := nativeRestoreSend
	nativeRestoreSend = func(id, text string, enter bool) error {
		err := send(id, text, enter)
		if id == "%7" && err == nil {
			change()
		}
		return err
	}
	previous := remoteRestoreProgress
	t.Cleanup(func() { remoteRestoreProgress = previous })
	var second resumeAttempt
	remoteRestoreProgress = func(_ string, attempts []resumeAttempt) error {
		for _, a := range attempts {
			if a.Pane == "%8" {
				second = a
			}
		}
		return nil
	}
	resumeAgentsContext(context.Background())
	data, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	lines := string(data)
	if strings.Count(lines, "send-keys -t %7 -l") != 1 || strings.Count(lines, "send-keys -t %7 Enter") != 1 {
		t.Fatalf("first dispatch: %s", lines)
	}
	if wantSecond {
		if strings.Count(lines, "send-keys -t %8 -l") != 1 || strings.Count(lines, "send-keys -t %8 Enter") != 1 {
			t.Fatalf("missing stable dispatch: %s", lines)
		}
	} else if strings.Contains(lines, "send-keys -t %8 ") || second.State != "target-changed" {
		t.Fatalf("changed target: receipt=%q calls=%s", second.State, lines)
	}
}

func TestQueuedHistoricalOwnership(t *testing.T) {
	for _, route := range []string{"native", "wrapper"} {
		for _, event := range []string{"stable", "second-match", "original-locator-live", "conversation-running"} {
			t.Run(route+"/"+event, func(t *testing.T) {
				a, b, calls := visibleRestoreFixture(t, false, nil)
				for _, loc := range []string{"pair:0.0", "pair:0.1", "worker:0.0"} {
					resume.Remove(loc)
				}
				if route == "wrapper" {
					b.Launcher = "fixture-wrapper"
				}
				save := func(loc string, rec resume.Record) {
					t.Helper()
					if err := resume.Save(loc, rec); err != nil {
						t.Fatal(err)
					}
				}
				save("old:0.0", a)
				save("old:0.1", b)
				observeQueuedRestore(t, calls, func() {
					switch event {
					case "second-match":
						other := b
						other.SessionID = "another-queued-thread"
						writeRecoveryFixture(t, other.SessionID, other.Cwd)
						save("another:0.1", other)
					case "original-locator-live":
						changeQueuedTmuxReply(t, "list-panes) ", `list-panes) printf '%%9\tbash\told:0.1\t%s\n' "$HOME"; `)
					case "conversation-running":
						save("occupied:1.0", b)
						changeQueuedTmuxReply(t, "list-panes) ", `list-panes) printf '%%9\tcodex\toccupied:1.0\t%s\n' "$HOME"; `)
					}
				}, event == "stable")
			})
		}
	}
}

func TestQueuedCompatibilityTarget(t *testing.T) {
	for _, event := range []string{"stable", "editor", "renamed", "unreadable"} {
		t.Run(event, func(t *testing.T) {
			_, b, calls := visibleRestoreFixture(t, false, nil)
			b.Launcher = "fixture-wrapper"
			if err := resume.Save("pair:0.1", b); err != nil {
				t.Fatal(err)
			}
			observeQueuedRestore(t, calls, func() {
				before := `*%8*) printf '%%8\tpair:0.1\tbash\n'`
				switch event {
				case "editor":
					changeQueuedTmuxReply(t, before, `*%8*) printf '%%8\tpair:0.1\tvim\n'`)
				case "renamed":
					changeQueuedTmuxReply(t, before, `*%8*) printf '%%8\tother:0.1\tbash\n'`)
				case "unreadable":
					changeQueuedTmuxReply(t, before, "*%8*) exit 1")
				}
			}, event == "stable")
		})
	}
}
