package codexterminal

import (
	"context"
	"strings"
	"testing"
)

func TestCFR008MissingAncestorIsUnavailableAndACompleteRecheckCanSucceed(t *testing.T) {
	oldRead, oldStdin := launchRead, launchStdin
	t.Cleanup(func() { launchRead, launchStdin = oldRead, oldStdin })
	for _, middle := range []bool{false, true} {
		t.Run(map[bool]string{false: "known-bridge", true: "unlisted-intermediate"}[middle], func(t *testing.T) {
			full := fixture
			missing := 200
			if middle {
				full = strings.Replace(full, "300 200 ", "300 250 ", 1) + "\n250 200 200 200 ttys001 Mon Oct 19 00:00:02 2026 /bin/sh\n"
				missing = 250
			}
			calls := 0
			launchRead = func(_ context.Context, name string, _ ...string) string {
				if name != "ps" {
					return "%7\t100\t/dev/ttys001\tsample-launcher\t/synthetic\ttest:0.0"
				}
				calls++
				if calls != 1 {
					return full
				}
				return omitProcess(full, missing)
			}
			launchStdin = func(context.Context, int) string { return "/dev/ttys001" }
			p := Pane{ID: "%7", PID: 100, Command: "sample-launcher", TTY: "/dev/ttys001", Cwd: "/synthetic", Loc: "test:0.0"}
			unavailable := ObserveLaunch(context.Background(), p, 300, 200, 300)
			if unavailable.Code != "process-unavailable" || !unavailable.Retryable() || unavailable.PID != 0 {
				t.Fatal("partial evidence became a hard conflict/authority", unavailable)
			}
			ready := ObserveLaunch(context.Background(), p, 300, 200, 300)
			if ready.Code != "" || ready.PID != 300 || len(ready.Ancestors) < 3 || calls != 3 {
				t.Fatal("complete recheck failed", ready, calls)
			}
		})
	}
}

func TestCFR008ObservedConflictsStayHardWhenAnotherRequiredRowIsMissing(t *testing.T) {
	oldRead, oldStdin := launchRead, launchStdin
	t.Cleanup(func() { launchRead, launchStdin = oldRead, oldStdin })
	for _, kind := range []string{"peer-tty", "peer-foreground", "ambiguous", "birth-after-missing"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			child := 300
			launchRead = func(_ context.Context, name string, _ ...string) string {
				if name != "ps" {
					return "%7\t100\t/dev/ttys001\tsample-launcher\t/synthetic\ttest:0.0"
				}
				calls++
				raw := fixture
				switch kind {
				case "peer-tty":
					raw = omitProcess(raw, 200)
					raw = strings.Replace(raw, "300 200 200 200 ttys001", "300 200 200 200 ttys002", 1)
				case "peer-foreground":
					raw = omitProcess(raw, 200)
					raw = strings.Replace(raw, "300 200 200 200 ttys001", "300 200 999 200 ttys001", 1)
				case "ambiguous":
					raw += "\n301 200 200 200 ttys001 Mon Oct 19 00:00:02 2026 /opt/codex\n"
				case "birth-after-missing":
					if calls == 2 {
						raw = omitProcess(raw, 200)
						raw = strings.Replace(raw, "300 200 200 200 ttys001 Mon Oct 19 00:00:02 2026", "300 200 200 200 ttys001 Mon Oct 19 00:00:09 2026", 1)
					}
				}
				return raw
			}
			if kind == "ambiguous" {
				child = 400
			} // absent launch row must not mask two observed candidates
			launchStdin = func(context.Context, int) string { return "/dev/ttys001" }
			got := ObserveLaunch(context.Background(), Pane{ID: "%7", PID: 100, Command: "sample-launcher", TTY: "/dev/ttys001", Cwd: "/synthetic", Loc: "test:0.0"}, child, 200, 300)
			if got.Code == "" || got.Retryable() || got.PID != 0 {
				t.Fatal("observed conflict weakened", kind, got)
			}
			if kind == "ambiguous" && got.Code != "ambiguous-client" {
				t.Fatal(got)
			}
			if kind == "birth-after-missing" && got.Code != "process-changed" {
				t.Fatal(got)
			}
		})
	}
}

func omitProcess(raw string, pid int) string {
	prefix := map[int]string{200: "200 ", 250: "250 "}[pid]
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), prefix) {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
