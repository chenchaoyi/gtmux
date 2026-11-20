package app

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

func serialSourceFixture(ctx context.Context, mode resumeMode, slots int) (*restoreSerialGate, *time.Time) {
	clock := time.Unix(1000, 0)
	g := newRestoreSerialGate(ctx, mode, slots)
	g.now = func() time.Time { return clock }
	g.pause = func(_ context.Context, d time.Duration) bool { clock = clock.Add(d); return true }
	g.observe = func(resumeAttempt) bool { return false }
	return g, &clock
}

func TestRestoreSerialWaitsForReadyBeforeNextStart(t *testing.T) {
	g, _ := serialSourceFixture(context.Background(), resumeAuto, 2)
	var events []string
	checks := 0
	g.observe = func(a resumeAttempt) bool {
		if a.State != "pending" {
			t.Fatal("native confirmation lost pending CAS input")
		}
		if a.Pane == "A" {
			if strings.Contains(strings.Join(events, ","), "start-B") {
				t.Fatal("B started while A was not ready")
			}
			checks++
			if checks < 2 {
				return false
			}
			events = append(events, "ready-A")
		}
		return true
	}
	for _, pane := range []string{"A", "B"} {
		a, sent := g.run(func() (resumeAttempt, bool) {
			events = append(events, "start-"+pane)
			return resumeAttempt{Pane: pane, State: "pending"}, true
		})
		if !sent || a.State != "confirmed" {
			t.Fatalf("not confirmed: %+v", a)
		}
	}
	if strings.Join(events, ",") != "start-A,ready-A,start-B" || checks != 2 {
		t.Fatal(events, checks)
	}
}

func TestRestoreSerialTimeoutVisibleBeforeNextStartAndBudgetShared(t *testing.T) {
	g, clock := serialSourceFixture(context.Background(), resumeAuto, 2)
	began := *clock
	var terminal []string
	g.publish = func(a resumeAttempt) {
		if a.State != "pending" {
			terminal = append(terminal, a.Pane+":"+a.State)
		}
	}
	for _, pane := range []string{"A", "B"} {
		a, _ := g.run(func() (resumeAttempt, bool) {
			if pane == "B" && (len(terminal) != 1 || terminal[0] != "A:startup-timeout") {
				t.Fatal("B preceded visible A timeout", terminal)
			}
			return resumeAttempt{Pane: pane, State: "pending"}, true
		})
		if a.State != "startup-timeout" {
			t.Fatal(a)
		}
	}
	if clock.Sub(began) != restoreSerialBudget || g.remaining != 0 {
		t.Fatal("per-pane waits expanded batch budget", clock.Sub(began), g.remaining)
	}
	if strings.Join(terminal, ",") != "A:startup-timeout,B:startup-timeout" {
		t.Fatal(terminal)
	}
}

func TestRestoreSerialCancelStopsFurtherStarts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g, _ := serialSourceFixture(ctx, resumeAuto, 2)
	starts := 0
	g.pause = func(context.Context, time.Duration) bool { cancel(); return false }
	for _, pane := range []string{"A", "B"} {
		a, _ := g.run(func() (resumeAttempt, bool) {
			starts++
			if pane == "B" {
				t.Fatal("cancelled queue started B")
			}
			return resumeAttempt{Pane: pane, State: "pending"}, true
		})
		if pane == "A" && a.State != "cancelled" {
			t.Fatal(a)
		}
	}
	if starts != 1 || !g.stopped {
		t.Fatal("queue ignored cancel")
	}
	pre, stop := context.WithCancel(context.Background())
	stop()
	g, _ = serialSourceFixture(pre, resumeAuto, 1)
	_, _ = g.run(func() (resumeAttempt, bool) { t.Fatal("pre-cancelled start"); return resumeAttempt{}, false })
}

func TestRestoreSerialTypeAndUnavailableDoNotWait(t *testing.T) {
	for _, tc := range []struct {
		mode  resumeMode
		state string
	}{{resumeType, "prefilled"}, {resumeAuto, "missing"}, {resumeAuto, "already-attempted"}} {
		g, _ := serialSourceFixture(context.Background(), tc.mode, 1)
		g.observe = func(resumeAttempt) bool { t.Fatal("unavailable/type observed as startup"); return false }
		g.pause = func(context.Context, time.Duration) bool { t.Fatal("unavailable/type waited"); return false }
		a, _ := g.run(func() (resumeAttempt, bool) {
			return resumeAttempt{Pane: "A", State: tc.state}, tc.state == "prefilled"
		})
		if a.State != tc.state || g.remaining != restoreSerialBudget {
			t.Fatal(a, g.remaining)
		}
	}
}

func TestRestoreSerialStableLocatorThenPaneOrder(t *testing.T) {
	panes := []struct{ loc, pane string }{{"z:0.0", "%1"}, {"a:2.0", "%3"}, {"a:10.0", "%4"}, {"a:2.0", "%2"}}
	sort.SliceStable(panes, func(i, j int) bool {
		return restoreSerialLess(panes[i].loc, panes[i].pane, panes[j].loc, panes[j].pane)
	})
	var got []string
	for _, p := range panes {
		got = append(got, p.loc+"/"+p.pane)
	}
	if strings.Join(got, ",") != "a:10.0/%4,a:2.0/%2,a:2.0/%3,z:0.0/%1" {
		t.Fatal(got)
	}
}

// serialNativeCommandMatches pins the actual executable/argv and keeps cwd/Home
// as data. A directory named gtmux, frontend, or bridge is not a launch target.
func serialNativeCommandMatches(command, cwd, home string, argv []string) bool {
	if len(argv) != 5 || !filepath.IsAbs(argv[0]) || filepath.Base(argv[0]) != "codex" ||
		argv[1] != "--no-alt-screen" || argv[2] != "--no-daemon" || argv[3] != "resume" || argv[4] == "" {
		return false
	}
	quote := func(word string) string { return "'" + strings.ReplaceAll(word, "'", "'\\''") + "'" }
	words := make([]string, len(argv))
	for i, word := range argv {
		words[i] = quote(word)
	}
	want := "cd -- " + quote(cwd) + " && env CODEX_HOME=" + quote(home) + " " + strings.Join(words, " ")
	return command == want
}

func TestRestoreSerialNativeIssueKeepsCommandEnterAndStoreOrder(t *testing.T) {
	// Explicit positive and negative contract cases; do not make test temp paths
	// avoid the words whose presence caused the original false positive.
	cwd, home := "/work/gtmux frontend bridge", "/home/gtmux frontend bridge"
	argv := []string{"/vendor/gtmux/bin/codex", "--no-alt-screen", "--no-daemon", "resume", "cnr-session"}
	command := "cd -- '/work/gtmux frontend bridge' && env CODEX_HOME='/home/gtmux frontend bridge' '/vendor/gtmux/bin/codex' '--no-alt-screen' '--no-daemon' 'resume' 'cnr-session'"
	if !serialNativeCommandMatches(command, cwd, home, argv) {
		t.Fatal("directory names changed native target classification")
	}
	for _, target := range []string{"gtmux", "frontend", "bridge"} {
		bad := strings.Replace(command, "'/vendor/gtmux/bin/codex'", "'/bin/"+target+"'", 1)
		if serialNativeCommandMatches(bad, cwd, home, argv) {
			t.Fatalf("accepted %s execution target", target)
		}
		badArgv := append([]string(nil), argv...)
		badArgv[0] = "/bin/" + target
		if serialNativeCommandMatches(bad, cwd, home, badArgv) {
			t.Fatalf("accepted %s argv target", target)
		}
	}
	for _, flag := range []string{"--frontend", "--bridge"} {
		bad := strings.Replace(command, "'resume'", "'"+flag+"' 'resume'", 1)
		if serialNativeCommandMatches(bad, cwd, home, argv) {
			t.Fatalf("accepted extra bridge routing %s", flag)
		}
	}
	plan, _, _ := cnrAppPlan(t)
	var events []string
	used := map[string]bool{}
	nativeRestoreReserve = func(id resume.NativeRestoreIdentity) (resume.NativeRestoreAttempt, error) {
		events = append(events, "reserve")
		return resume.NativeRestoreAttempt{State: "reserved", Identity: id}, nil
	}
	nativeRestoreSend = func(pane, command string, enter bool) error {
		if strings.Join(events, ",") != "reserve" || !used[plan.Record.SessionID] {
			t.Fatal("send preceded reservation")
		}
		if pane != plan.Pane.Pane || command != plan.Command || !enter {
			t.Fatal("native command/Enter changed")
		}
		argv := []string{plan.Binary.Physical, "--no-alt-screen", "--no-daemon", "resume", plan.Record.SessionID}
		if !serialNativeCommandMatches(command, plan.Record.Cwd, plan.Identity.Home, argv) {
			t.Fatal("default native route changed", command)
		}
		events = append(events, "send")
		return nil
	}
	nativeRestoreUpdate = func(a resume.NativeRestoreAttempt, next string) (resume.NativeRestoreAttempt, error) {
		events = append(events, next)
		a.State = next
		return a, nil
	}
	g, _ := serialSourceFixture(context.Background(), resumeAuto, 2)
	g.observe = func(a resumeAttempt) bool {
		if strings.Join(events, ",") != "reserve,send,pending" || a.Native == nil || a.Native.State != "pending" {
			t.Fatal("observation preceded stored pending", events)
		}
		return true // isolated readiness seam, not a real native confirmation result
	}
	a, _ := g.run(func() (resumeAttempt, bool) { return issueNativeRestoreContext(g.ctx, plan, resumeAuto, used) })
	if a.State != "confirmed" {
		t.Fatal(a)
	}
	_, sent := g.run(func() (resumeAttempt, bool) { return issueNativeRestoreContext(g.ctx, plan, resumeAuto, used) })
	if sent || strings.Join(events, ",") != "reserve,send,pending" {
		t.Fatal("same session replayed", events)
	}
}

func TestRestoreSerialCancelAfterReserveKeepsOccupationWithoutSend(t *testing.T) {
	plan, _, _ := cnrAppPlan(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	used := map[string]bool{}
	nativeRestoreReserve = func(id resume.NativeRestoreIdentity) (resume.NativeRestoreAttempt, error) {
		cancel()
		return resume.NativeRestoreAttempt{State: "reserved", Identity: id}, nil
	}
	nativeRestoreSend = func(string, string, bool) error { t.Fatal("sent after cancellation"); return nil }
	updates := 0
	nativeRestoreUpdate = func(a resume.NativeRestoreAttempt, next string) (resume.NativeRestoreAttempt, error) {
		if next != "not-sent" {
			t.Fatal(next)
		}
		updates++
		a.State = next
		return a, nil
	}
	a, sent := issueNativeRestoreContext(ctx, plan, resumeAuto, used)
	if sent || a.State != "cancelled" || !used[plan.Record.SessionID] || updates != 1 {
		t.Fatal(a, sent, used, updates)
	}
}

func TestRestoreSerialUncertainSendWaitsWithoutClaimingReady(t *testing.T) {
	for _, state := range []string{"unknown", "send-failed"} {
		g, clock := serialSourceFixture(context.Background(), resumeAuto, 2)
		began := *clock
		g.observe = func(resumeAttempt) bool {
			t.Fatal("uncertain delivery cannot be confirmed through pending CAS")
			return true
		}
		a, _ := g.run(func() (resumeAttempt, bool) { return resumeAttempt{Pane: "A", State: state}, false })
		if a.State != "startup-timeout" || a.IssueState != state || clock.Sub(began) != restoreSerialBudget/2 {
			t.Fatal(a, clock.Sub(began))
		}
		_, _ = g.run(func() (resumeAttempt, bool) {
			if a.State != "startup-timeout" {
				t.Fatal("B preceded uncertain A timeout")
			}
			return resumeAttempt{Pane: "B", State: "missing"}, false
		})
	}
}
