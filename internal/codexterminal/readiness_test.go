package codexterminal

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestCFR001ReadinessPreservesStrictTerminalProof(t *testing.T) {
	for _, kind := range []string{"ready", "snapshot", "stdin-unavailable", "stdin-mismatch", "birth", "pane", "ambiguous", "foreground"} {
		t.Run(kind, func(t *testing.T) {
			before, after := parseProcesses(fixture), parseProcesses(fixture)
			p := Pane{ID: "%7", PID: 100, Command: "sample-launcher", TTY: "/dev/ttys001"}
			want, retry := "", false
			switch kind {
			case "snapshot":
				before = nil
				want, retry = "snapshot-unavailable", true
			case "stdin-unavailable":
				want, retry = "stdin-unavailable", true
			case "stdin-mismatch":
				want = "terminal-mismatch"
			case "birth":
				c := after[300]
				c.start = "new"
				after[300] = c
				want = "process-changed"
			case "pane":
				want = "pane-changed"
			case "ambiguous":
				c := before[300]
				c.pid = 301
				before[301] = c
				after[301] = c
				want = "ambiguous-client"
			case "foreground":
				c := before[300]
				c.group = 999
				before[300] = c
				after[300] = c
				want = "client-unverified"
			}
			calls := 0
			i := inspector{snapshot: func() map[int]process {
				calls++
				if calls == 1 {
					return before
				}
				return after
			},
				stdin: func(int) string {
					if kind == "stdin-unavailable" {
						return ""
					}
					if kind == "stdin-mismatch" {
						return "/dev/ttys002"
					}
					return "/dev/ttys001"
				},
				pane: func(Pane) bool { return kind != "pane" }}
			o := i.observe([]Pane{p})[p.ID]
			if o.Code != want || o.Retryable() != retry {
				t.Fatalf("observation=%+v want %q retry=%v", o, want, retry)
			}
			if want == "" && (o.PID != 300 || !reflect.DeepEqual(o.Ancestors, []int{100, 200, 300}) || o.Start == "" || o.RootStart == "" || o.TTY != "ttys001") {
				t.Fatalf("incomplete proof: %+v", o)
			}
			if want != "" && o.PID != 0 {
				t.Fatal("refusal leaked a verified client", o)
			}
		})
	}
}

func TestCFR001DirectLaunchScopeAndParentCancellation(t *testing.T) {
	procs := parseProcesses(fixture)
	if !directLaunch(procs, 100, 200, 300) || directLaunch(procs, 100, 200, 400) || directLaunch(procs, 100, 999, 300) {
		t.Fatal("direct launch proof changed")
	}
	oldRead, oldStdin := launchRead, launchStdin
	t.Cleanup(func() { launchRead, launchStdin = oldRead, oldStdin })
	for _, wrapped := range []bool{false, true} {
		psCalls, stdinCalls := 0, 0
		launchRead = func(ctx context.Context, name string, args ...string) string {
			if ctx.Err() != nil {
				return ""
			}
			if name == "ps" {
				psCalls++
				want := []string{"-p", "100,200,300", "-o", "pid=,ppid=,pgid=,tpgid=,tty=,lstart=,comm="}
				if wrapped {
					want = []string{"-axo", "pid=,ppid=,pgid=,tpgid=,tty=,lstart=,comm="}
				}
				if !reflect.DeepEqual(args, want) {
					t.Fatalf("unexpected process scope: %v", args)
				}
				if wrapped {
					return fixture
				}
				return strings.Join(strings.Split(strings.TrimSpace(fixture), "\n")[:3], "\n")
			}
			return "%7\t100\t/dev/ttys001\tsample-launcher\t/synthetic\ttest:0.0"
		}
		launchStdin = func(context.Context, int) string { stdinCalls++; return "/dev/ttys001" }
		child := 300
		if wrapped {
			child = 200
		}
		got := ObserveLaunch(context.Background(), Pane{ID: "%7", PID: 100, Command: "sample-launcher", TTY: "/dev/ttys001", Cwd: "/synthetic", Loc: "test:0.0"}, child, 200, 300)
		if got.Code != "" || got.PID != 300 || psCalls != 2 || stdinCalls != 2 {
			t.Fatalf("incomplete launch check: %+v ps=%d stdin=%d", got, psCalls, stdinCalls)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	o := ObserveLaunch(ctx, Pane{ID: "%7", PID: 100, TTY: "/dev/ttys001"}, 300, 200, 300)
	if o.PID != 0 || o.Code != "snapshot-unavailable" {
		t.Fatal("cancelled observation accepted", o)
	}
}
