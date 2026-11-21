package app

import (
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func TestRestoreStartupSummaryKeepsAcceptedTimeoutPending(t *testing.T) {
	old := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(old) })
	attempts := []resumeAttempt{
		{Pane: "%1", State: "confirmed"},
		{Pane: "%2", State: "pending"},
		{Pane: "%3", Loc: "accepted:0.0", State: "startup-timeout", IssueState: "pending"},
		{Pane: "%4", Loc: "unknown:0.0", State: "startup-timeout", IssueState: "unknown"},
		{Pane: "%5", State: "send-failed"},
	}
	for _, tt := range []struct {
		lang, totals, acceptedLabel, unknownLabel string
	}{
		{"en", "1 confirmed · 2 pending · 2 skipped/failed", "Startup wait timed out", "Result unknown"},
		{"zh", "1 个已确认 · 2 个待确认 · 2 个跳过或失败", "启动等待已超时", "结果未知"},
	} {
		t.Run(tt.lang, func(t *testing.T) {
			i18n.SetLang(tt.lang)
			got := renderResumeAttempts(attempts, 100)
			if !strings.Contains(got, tt.totals) {
				t.Fatalf("accepted but unverified startup must remain pending:\n%s", got)
			}
			accepted := resumeAttemptLabel(attempts[2])
			unknown := resumeAttemptLabel(attempts[3])
			if !strings.Contains(accepted, tt.acceptedLabel) || strings.Contains(accepted, tt.unknownLabel) ||
				!strings.Contains(unknown, tt.acceptedLabel) || !strings.Contains(unknown, tt.unknownLabel) {
				t.Fatalf("accepted and uncertain outcomes became indistinguishable: %q, %q", accepted, unknown)
			}
		})
	}
}
