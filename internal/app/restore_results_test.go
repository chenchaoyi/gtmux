package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func writeRecoveryFixture(t *testing.T, id, cwd string) {
	t.Helper()
	dir := filepath.Join(transcript.CodexHome(), "sessions/2026/01/01")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{"type": "session_meta", "payload": map[string]string{"id": id, "cwd": cwd}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-time-"+id+".jsonl"), append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestResumeCommandIsNotAConfirmedConversation(t *testing.T) {
	r := resume.Record{Agent: "codex", SessionID: "target", HomeID: "home", Frontend: "old", UpdatedAt: 10}
	a := resumeAttempt{Record: r, Before: r, IssuedAt: 20, State: "pending"}
	if resumeAttemptConfirmed(a, r, true) {
		t.Fatal("old binding confirmed a new command")
	}
	current := r
	current.UpdatedAt = 21
	current.Frontend = "new"
	if !resumeAttemptConfirmed(a, current, true) {
		t.Fatal("fresh same-thread binding was not confirmed")
	}
	for _, modify := range []func(*resume.Record){func(r *resume.Record) { r.SessionID = "other" }, func(r *resume.Record) { r.HomeID = "other" }, func(r *resume.Record) { r.UpdatedAt = 19 }} {
		wrong := current
		modify(&wrong)
		if resumeAttemptConfirmed(a, wrong, true) {
			t.Fatal("wrong identity confirmed", wrong)
		}
	}
	if resumeAttemptConfirmed(a, current, false) {
		t.Fatal("expired live lease confirmed")
	}
}

func TestRestoreTerminalPreview(t *testing.T) {
	previous := i18n.Lang()
	i18n.SetLang("zh")
	t.Cleanup(func() { i18n.SetLang(previous) })
	saved := []windowShape{idShape("example", "1234,80x24,0,0,44", 1), idShape("hq", "1234,80x24,0,0,45", 1)}
	live := []windowShape{idShape("example", "1234,80x24,0,0,7", 1), idShape("hq", "1234,80x24,0,0,8", 1)}
	t.Log("\n" + renderRestorePaneIDs(saved, live, 80) + renderResumeAttempts([]resumeAttempt{
		{Loc: "example:0.0", Pane: "%7", State: "confirmed"},
		{Loc: "hq:0.0", Pane: "%8", State: "missing"},
	}, 80))
}

func TestRestoreResultsStayReadableAcrossLanguagesAndWidths(t *testing.T) {
	previous := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(previous) })
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		for _, width := range []int{20, 32, 60, 80, 120} {
			attempts := []resumeAttempt{{Loc: "hq:0.0", Pane: "%9", State: "missing"}, {Loc: "example:0.0", Pane: "%10", State: "pending"}, {Loc: "资料\n\x1b[31m:1.0", Pane: "%11", State: "confirmed"}}
			out := renderResumeAttempts(attempts, width)
			for _, line := range strings.Split(out, "\n") {
				if i18n.DispWidth(line) > min(width, 100) {
					t.Fatalf("%s/%d overflow %q", lang, width, line)
				}
			}
			if strings.ContainsAny(out, "\x1b\r") {
				t.Fatal("unsafe name rendered")
			}
			if !strings.Contains(out, resumeStateLabel("missing")) && width >= 60 {
				t.Fatal("skip reason missing")
			}
			if lang == "zh" && width == 80 {
				t.Log("\n" + out)
			}
		}
	}
}

func TestPaneReceiptIsCompactAndRetainsEveryIdentity(t *testing.T) {
	var saved, live []windowShape
	for i := 0; i < 20; i++ {
		saved = append(saved, idShape(fmt.Sprintf("example-%d", i), fmt.Sprintf("1234,80x24,0,0,%d", i+1), 1))
		live = append(live, idShape(fmt.Sprintf("example-%d", i), fmt.Sprintf("1234,80x24,0,0,%d", i+51), 1))
	}
	out := renderRestorePaneIDs(saved, live, 80)
	if strings.Count(out, "\n") > 26 {
		t.Fatal("receipt grew back into two rows per window")
	}
	for i := range saved {
		if !strings.Contains(out, fmt.Sprintf("%%%d → %%%d", i+1, i+51)) {
			t.Fatal("mapping lost", i)
		}
	}
	for _, width := range []int{20, 32, 60, 80} {
		for _, line := range strings.Split(renderRestorePaneIDs(saved, live, width), "\n") {
			if i18n.DispWidth(line) > width {
				t.Fatalf("overflow %d %q", width, line)
			}
		}
	}
	t.Log("\n" + out)
}
