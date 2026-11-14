package codexbridge

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

func TestCFL005HeartbeatFailuresAreBoundedAndRecoveryIsVisible(t *testing.T) {
	b, _, clock, calls := recoveryFixture(t)
	b.attachment.SessionID = "same-root"
	b.attachment.Loc = "old:0.1"
	var reports []string
	b.report = func(kind string, err error, _ int) {
		s := kind
		if err != nil {
			s += " " + err.Error()
		}
		reports = append(reports, s)
	}
	reason := "terminal-proof-unavailable"
	b.refresh = func(f resume.CodexFrontend) (resume.CodexFrontend, error) {
		*calls = append(*calls, "refresh")
		return f, &resume.FrontendBindingError{Code: reason, Retryable: true}
	}
	for i := 0; i < 10; i++ {
		b.tick()
		*clock = clock.Add(time.Second)
	}
	if len(reports) != 1 {
		t.Fatalf("one repeated heartbeat failure must produce one bounded diagnostic, got %v", reports)
	}
	if !strings.Contains(reports[0], reason) {
		t.Fatal("diagnostic omitted value-free cause", reports)
	}
	reason = "frontend-owner-changed"
	b.tick()
	if len(reports) != 2 {
		t.Fatal("a different cause was hidden by rate limiting", reports)
	}
	b.refresh = func(f resume.CodexFrontend) (resume.CodexFrontend, error) { f.Loc = "new:0.0"; return f, nil }
	b.tick()
	if b.attachment.SessionID != "same-root" || b.attachment.Loc != "new:0.0" {
		t.Fatal("migration changed root or failed to commit", b.attachment)
	}
	if len(reports) != 3 || !strings.Contains(reports[2], "ready") {
		t.Fatal("heartbeat recovery was silent", reports)
	}
	for _, c := range *calls {
		if strings.HasPrefix(c, "bind:") {
			t.Fatal("heartbeat retried a root RPC", *calls)
		}
	}
	b.close()
	n := len(reports)
	b.tick()
	b.stage("late-root", "")
	if len(reports) != n || b.attachment.SessionID != "same-root" {
		t.Fatal("retired lifetime resumed diagnostics or binding", reports)
	}
}

func TestCFL005HeartbeatDiagnosticsNeverPrintArbitraryRefreshError(t *testing.T) {
	b, _, _, _ := recoveryFixture(t)
	b.attachment.SessionID = "root"
	var report string
	b.report = func(kind string, err error, _ int) {
		report = kind
		if err != nil {
			report += " " + err.Error()
		}
	}
	b.refresh = func(f resume.CodexFrontend) (resume.CodexFrontend, error) {
		return f, errors.New("synthetic-private-content-do-not-log")
	}
	b.tick()
	if report == "" || strings.Contains(report, "synthetic-private-content-do-not-log") {
		t.Fatal("missing or content-leaking heartbeat diagnostic", report)
	}
}

func TestCFL005HeartbeatWindowCapsChangingCausesAndReportsRelocation(t *testing.T) {
	b, _, clock, _ := recoveryFixture(t)
	b.attachment.SessionID = "root"
	b.attachment.Loc = "old:0.0"
	var reports []string
	b.report = func(kind string, err error, _ int) {
		s := kind
		if err != nil {
			s += " " + err.Error()
		}
		reports = append(reports, s)
	}
	reason := "terminal-proof-unavailable"
	b.refresh = func(f resume.CodexFrontend) (resume.CodexFrontend, error) {
		*clock = clock.Add(5 * time.Millisecond)
		return f, &resume.FrontendBindingError{Code: reason, Retryable: true}
	}
	for _, cause := range []string{"terminal-proof-unavailable", "frontend-owner-changed", "backend-proof-unavailable", "terminal-changed"} {
		reason = cause
		b.tick()
	}
	if len(reports) != 2 {
		t.Fatal("changing causes exceeded per-window diagnostic cap", reports)
	}
	*clock = clock.Add(31 * time.Second)
	b.tick()
	if len(reports) != 3 || !strings.Contains(reports[2], "elapsed_ms=5") {
		t.Fatal("new window did not report measured failure", reports)
	}
	b.refresh = func(f resume.CodexFrontend) (resume.CodexFrontend, error) { return f, nil }
	b.tick()
	b.refresh = func(f resume.CodexFrontend) (resume.CodexFrontend, error) { f.Loc = "new:0.0"; return f, nil }
	b.tick()
	if len(reports) != 5 || reports[3] != "ready" || reports[4] != "relocated" {
		t.Fatal("recovery/normal relocation outcomes incorrect", reports)
	}
	b.refresh = func(f resume.CodexFrontend) (resume.CodexFrontend, error) { return f, errors.New("private path") }
	b.tick()
	if !strings.Contains(reports[len(reports)-1], "last_success_ms=") || strings.Contains(reports[len(reports)-1], "last_success_ms=0") {
		t.Fatal("last successful heartbeat was omitted", reports)
	}
	// Brief successful ticks cannot replenish a 30-second failure allowance.
	for i := 0; i < 8; i++ {
		b.refresh = func(f resume.CodexFrontend) (resume.CodexFrontend, error) { return f, nil }
		b.tick()
		b.refresh = func(f resume.CodexFrontend) (resume.CodexFrontend, error) { return f, errors.New("private") }
		b.tick()
	}
	failures := 0
	for _, r := range reports {
		if strings.HasPrefix(r, "heartbeat ") {
			failures++
		}
	}
	if failures != 4 {
		t.Fatal("flapping success replenished the failure log allowance", failures, reports)
	}

}
