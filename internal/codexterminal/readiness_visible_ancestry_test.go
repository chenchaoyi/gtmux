package codexterminal

import (
	"context"
	"strings"
	"testing"
)

func TestCFR010VisibleOutsideAncestryMustStayHard(t *testing.T) {
	oldRead, oldStdin := launchRead, launchStdin
	t.Cleanup(func() { launchRead, launchStdin = oldRead, oldStdin })
	for _, absentBridge := range []bool{false, true} {
		t.Run(map[bool]string{false: "outside-complete", true: "outside-missing-bridge"}[absentBridge], func(t *testing.T) {
			raw := strings.Replace(fixture, "300 200 ", "300 1 ", 1)
			if absentBridge {
				raw = omitProcess(raw, 200)
			}
			launchRead = func(_ context.Context, name string, _ ...string) string {
				if name == "ps" {
					return raw
				}
				return "%7\t100\t/dev/ttys001\tsample-launcher\t/synthetic\ttest:0.0"
			}
			launchStdin = func(context.Context, int) string { return "/dev/ttys001" }
			got := ObserveLaunch(context.Background(), Pane{ID: "%7", PID: 100, Command: "sample-launcher", TTY: "/dev/ttys001", Cwd: "/synthetic", Loc: "test:0.0"}, 300, 200, 300)
			t.Logf("observation=%+v retry=%v", got, got.Retryable())
			if got.Code == "" || got.Retryable() || got.PID != 0 {
				t.Fatalf("visible peer parent=1 outside pane=100 became retryable because unrelated bridge=200 was absent: %+v", got)
			}
		})
	}
}

func TestCFR010VisibleCycleStaysHardWithMissingBridge(t *testing.T) {
	oldRead, oldStdin := launchRead, launchStdin
	t.Cleanup(func() { launchRead, launchStdin = oldRead, oldStdin })
	raw := strings.Replace(fixture, "300 200 ", "300 250 ", 1) + "\n250 300 200 200 ttys001 Mon Oct 19 00:00:02 2026 /bin/sh\n"
	raw = omitProcess(raw, 200)
	launchRead = func(_ context.Context, name string, _ ...string) string {
		if name == "ps" {
			return raw
		}
		return "%7\t100\t/dev/ttys001\tsample-launcher\t/synthetic\ttest:0.0"
	}
	launchStdin = func(context.Context, int) string { return "/dev/ttys001" }
	got := ObserveLaunch(context.Background(), Pane{ID: "%7", PID: 100, Command: "sample-launcher", TTY: "/dev/ttys001", Cwd: "/synthetic", Loc: "test:0.0"}, 300, 200, 300)
	if got.Code == "" || got.Retryable() || got.PID != 0 {
		t.Fatal("visible ancestry cycle masked by missing bridge", got)
	}
}
