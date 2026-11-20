package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestRSDHistoricalSourceBeforeDispatch(t *testing.T) {
	for _, changed := range []bool{false, true} {
		name := "stable"
		if changed {
			name = "replaced"
		}
		t.Run(name, func(t *testing.T) {
			a, b, calls := visibleRestoreFixture(t, false, nil)
			for _, loc := range []string{"pair:0.0", "pair:0.1", "worker:0.0"} {
				resume.Remove(loc)
			}
			for loc, rec := range map[string]resume.Record{"old:0.0": a, "old:0.1": b} {
				if err := resume.Save(loc, rec); err != nil {
					t.Fatal(err)
				}
			}
			original := nativeRestoreSend
			nativeRestoreSend = func(target, text string, enter bool) error {
				err := original(target, text, enter)
				if changed && target == "%7" {
					if e := resume.Save("old:0.1", a); e != nil {
						t.Fatal(e)
					}
				}
				return err
			}
			resumeAgentsContext(context.Background())
			body, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			log := string(body)
			if strings.Count(log, "send-keys -t %7 ") != 2 {
				t.Fatalf("first target: %s", log)
			}
			want := 2
			if changed {
				want = 0
			}
			if strings.Count(log, "send-keys -t %8 ") != want || (!changed && !strings.Contains(log, b.SessionID)) {
				t.Fatalf("historical source changed=%v: %s", changed, log)
			}
		})
	}
}

func TestRSDCompatibilityUncertainEnter(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		name := "distinct"
		if duplicate {
			name = "duplicate"
		}
		t.Run(name, func(t *testing.T) {
			a, b, calls := visibleRestoreFixture(t, false, nil)
			if duplicate {
				b = a
			}
			for loc, rec := range map[string]resume.Record{"pair:0.0": a, "pair:0.1": b} {
				rec.Launcher = "fixture-wrapper"
				if err := resume.Save(loc, rec); err != nil {
					t.Fatal(err)
				}
			}
			stub := tmux.Bin
			if filepath.Dir(stub) != os.Getenv("HOME") || filepath.Base(stub) != "tmux" {
				t.Fatal("unexpected fixture sender")
			}
			script, err := os.ReadFile(stub)
			if err != nil {
				t.Fatal(err)
			}
			inject := strings.Replace(string(script), "shift\n", "shift\nif [ \"$*\" = 'send-keys -t %7 Enter' ]; then printf '%s\\n' \"$*\" >> \"$VISIBILITY_CALLS\"; exit 1; fi\n", 1)
			if err := os.WriteFile(stub, []byte(inject), 0700); err != nil {
				t.Fatal(err)
			}
			previous := remoteRestoreProgress
			t.Cleanup(func() { remoteRestoreProgress = previous })
			var first resumeAttempt
			remoteRestoreProgress = func(_ string, attempts []resumeAttempt) error {
				for _, a := range attempts {
					if a.Pane == "%7" {
						first = a
					}
				}
				return nil
			}
			resumeAgentsContext(context.Background())
			body, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			log := string(body)
			wantSecond := 2
			if duplicate {
				wantSecond = 0
			}
			if strings.Count(log, "send-keys -t %7 -l") != 1 || strings.Count(log, "send-keys -t %7 Enter") != 1 || strings.Count(log, "send-keys -t %8 ") != wantSecond || first.IssueState != "unknown" {
				t.Fatalf("compatibility receipt=%q calls=%s", first.IssueState, log)
			}
		})
	}
}

func TestRSDLocalArchiveBoundaries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg"))
	t.Setenv("GTMUX_LANG", "en")
	path := filepath.Join(home, "save")
	row := savePaneCmd("large", "0", "0", home, "bash", ":bash") + "\n"
	block := strings.Repeat("x", 32767) + "\n"
	body := row + strings.Repeat(block, (8<<20)/len(block))
	body = body[:8<<20] + "\n"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	p := buildRestorePlanFrom(path)
	if len(p.Sessions) != 1 || p.Sessions[0].Name != "large" {
		t.Error("valid large archive lost its session")
	}
	if _, err := remoteRestorePlanWith(path, func(...string) (string, error) {
		t.Fatal("remote oversized archive reached pane lookup")
		return "", nil
	}, time.Now()); err == nil {
		t.Fatal("remote preview lost its size limit")
	}
	t.Run("remote worker still bounded", func(t *testing.T) {
		prev := remoteRestoreSessions
		remoteRestoreSessions = map[string]bool{"large": true}
		t.Cleanup(func() { remoteRestoreSessions = prev })
		if loadResumeLayout(path).ReadError == nil {
			t.Error("remote worker accepted a large archive")
		}
		if _, err := restoreArchiveDigest(path); err == nil {
			t.Error("remote recheck accepted a large archive")
		}
	})
	first := loadSavedLayout(path)
	if err := os.WriteFile(path, []byte(body+"tail\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if string(first.Digest) == string(loadSavedLayout(path).Digest) {
		t.Error("archive tail missing from selection fingerprint")
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 1<<20+1)), 0600); err != nil {
		t.Fatal(err)
	}
	failed := renderRestorePlan(buildRestorePlanFrom(path), 80, true)
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	empty := renderRestorePlan(buildRestorePlanFrom(path), 80, true)
	if empty == failed || failed == "" {
		t.Errorf("read failure indistinguishable from empty archive: %q", failed)
	}
}
