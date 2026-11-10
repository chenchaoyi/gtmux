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
				want := []string{"-axo", "pid=,ppid=,pgid=,tpgid=,tty=,lstart=,comm="}
				if !reflect.DeepEqual(args, want) {
					t.Fatalf("unexpected process scope: %v", args)
				}
				return fixture
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

func TestCFR005DirectLaunchRetainsFullTreeAndMissingEvidenceRefuses(t *testing.T) {
	oldRead, oldStdin := launchRead, launchStdin
	t.Cleanup(func() { launchRead, launchStdin = oldRead, oldStdin })
	for _, kind := range []string{"ready", "sibling-before", "sibling-after", "peer-before", "peer-after", "changed-birth"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			launchRead = func(_ context.Context, name string, args ...string) string {
				if name != "ps" {
					return "%7\t100\t/dev/ttys001\tsample-launcher\t/synthetic\ttest:0.0"
				}
				calls++
				full := len(args) > 0 && args[0] == "-axo"
				raw := fixture
				if full && ((kind == "sibling-before") || (kind == "sibling-after" && calls == 2)) {
					raw += "\n301 200 200 200 ttys001 Mon Sep 28 01:02:03 2026 /opt/codex\n"
				}
				if (kind == "peer-before" && calls == 1) || (kind == "peer-after" && calls == 2) {
					lines := strings.Split(raw, "\n")
					var keep []string
					for _, line := range lines {
						if !strings.HasPrefix(strings.TrimSpace(line), "300 ") {
							keep = append(keep, line)
						}
					}
					raw = strings.Join(keep, "\n")
				}
				if kind == "changed-birth" && calls == 2 {
					raw = strings.Replace(raw, "300 200 200 200 ttys001 Mon Oct 19 00:00:02 2026", "300 200 200 200 ttys001 Mon Oct 19 00:00:09 2026", 1)
				}
				return raw
			}
			launchStdin = func(context.Context, int) string { return "/dev/ttys001" }
			p := Pane{ID: "%7", PID: 100, Command: "sample-launcher", TTY: "/dev/ttys001", Cwd: "/synthetic", Loc: "test:0.0"}
			o := ObserveLaunch(context.Background(), p, 300, 200, 300)
			switch kind {
			case "ready":
				if o.PID != 300 || o.Code != "" {
					t.Fatal(o)
				}
			case "sibling-before", "sibling-after":
				if o.Code != "ambiguous-client" || o.PID != 0 || o.Retryable() {
					t.Fatal("sibling admitted", kind, o)
				}
			case "peer-before", "peer-after":
				if o.Code != "process-unavailable" || !o.Retryable() || o.PID != 0 {
					t.Fatal("missing evidence misclassified", kind, o)
				}
			default:
				if o.Code != "process-changed" || o.Retryable() {
					t.Fatal(o)
				}
			}
		})
	}
}
