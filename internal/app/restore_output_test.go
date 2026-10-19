package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// Synthetic prompts only: these examples must never copy an operator's workspace.
func outputPlan() restorePlan {
	return restorePlan{Sessions: []restorePlanSession{
		{Name: "alpha", Windows: 2, Panes: 3, Agents: []restorePlanAgent{
			{Agent: "codex", Loc: "alpha:0.0", Goal: "Review the example module", Alive: true},
			{Agent: "codex", Loc: "alpha:1.0", Goal: "A long prompt " + strings.Repeat("sample中文", 40), Alive: true},
		}},
		{Name: "资料 workspace", Windows: 1, Panes: 2, Agents: []restorePlanAgent{
			{Agent: "claude", Loc: "资料 workspace:0.1", Cwd: "/example/project", Alive: false},
		}},
		{Name: "shell", Windows: 1, Panes: 1},
	}}
}

func TestRestoreOutputCompactAndDetailed(t *testing.T) {
	old := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(old) })
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		compact := renderRestorePlan(outputPlan(), 80, false)
		detailed := renderRestorePlan(outputPlan(), 80, true)
		if strings.Contains(compact, "Review the example") || strings.Contains(compact, "/example/project") || strings.Contains(compact, "A long prompt") {
			t.Fatalf("%s: normal progress leaked conversation detail:\n%s", lang, compact)
		}
		for _, want := range []string{"alpha", "资料 workspace", "shell", "codex ×2", "gtmux restore --plan"} {
			if !strings.Contains(compact, want) {
				t.Errorf("%s: compact output lost %q:\n%s", lang, want, compact)
			}
		}
		if !strings.Contains(compact, i18n.Tr("3 sessions · 2 agents resumable", "3 个会话 · 2 个 agent 可接回")) || !strings.Contains(compact, i18n.Tr("Skipped: 1", "跳过：1")) {
			t.Fatalf("%s: missing transcripts must not inflate the resumable total:\n%s", lang, compact)
		}
		for _, want := range []string{"↻ codex [0.0]  Review the example module", "× claude [0.1]  /example/project", "A long prompt"} {
			if !strings.Contains(detailed, want) {
				t.Errorf("%s: detailed preview lost %q:\n%s", lang, want, detailed)
			}
		}
		if strings.Count(compact, "\n") >= strings.Count(detailed, "\n") {
			t.Fatal("normal output must stay shorter than the detailed preview")
		}
		if lang == "zh" {
			t.Logf("Synthetic compact preview:\n%s", compact)
		}
	}
}

func TestRestoreOutputWidthAndSingleLines(t *testing.T) {
	p := outputPlan()
	p.Sessions[0].Name = "长会话名" + strings.Repeat("中文", 60) + "\n\x1b[31m"
	p.Sessions[0].Agents[0].Goal = "review\n\t\x1b[31m\u009b\r" + strings.Repeat("中文", 60)
	for _, width := range []int{20, 32, 40, 60, 80, 100, 240} {
		for _, details := range []bool{false, true} {
			out := renderRestorePlan(p, width, details)
			for _, line := range strings.Split(out, "\n") {
				if i18n.DispWidth(line) > min(width, 100) {
					t.Fatalf("width=%d: line overflow: %q", width, line)
				}
			}
			if strings.ContainsAny(out, "\x1b\u009b\r\t") || strings.Contains(out, "review\n") {
				t.Fatalf("raw control characters or embedded rows: %q", out)
			}
		}
	}
}

func TestRestoreOutputPlanCommandIsReadOnlyAndJSONUnchanged(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", "")
	calls := filepath.Join(t.TempDir(), "calls")
	t.Setenv("RESTORE_TEST_CALLS", calls)
	useDoctorTmuxStub(t, "echo unexpected > \"$RESTORE_TEST_CALLS\"; exit 1\n")
	t.Cleanup(func() {
		if _, err := os.Stat(calls); !os.IsNotExist(err) {
			t.Error("read-only preview invoked tmux")
		}
	})
	save := filepath.Join(state.Home(), ".tmux/resurrect/last")
	if err := os.MkdirAll(filepath.Dir(save), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(save, []byte(savePaneCmd("example", "0", "0", "/example", "codex", ":codex")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := resume.Save("example:0.0", resume.Record{Agent: "codex", SessionID: "synthetic-session", Cwd: "/example"}); err != nil {
		t.Fatal(err)
	}
	var rc int
	out := captureStdout(t, func() { rc = cmdRestore([]string{"--plan"}) })
	if rc != 0 || !strings.Contains(out, "↻ codex [0.0]  /example") {
		t.Fatalf("explicit preview lost its agent detail: rc=%d %s", rc, out)
	}
	out = captureStdout(t, func() { rc = cmdRestore([]string{"--plan", "--json"}) })
	var plan restorePlan
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatal(err)
	}
	if rc != 0 || plan.SavePath != save || len(plan.Sessions) != 1 || len(plan.Sessions[0].Agents) != 1 {
		t.Fatalf("JSON plan changed: %s", out)
	}
	a := plan.Sessions[0].Agents[0]
	if a.SessionID != "synthetic-session" || a.Loc != "example:0.0" || a.Cwd != "/example" || !a.Alive {
		t.Fatalf("JSON agent changed: %+v", a)
	}
}

func outputDrift() []layoutDifference {
	var saved, live []windowShape
	for i := 0; i < 8; i++ {
		saved = append(saved, windowShape{Session: fmt.Sprintf("example-%d", i), Index: "0", Panes: 4, Layout: "saved-geometry"})
		live = append(live, windowShape{Session: fmt.Sprintf("example-%d", i), Index: "0", Panes: 2, Layout: "live-geometry"})
	}
	live[1].Panes = 4 // Geometry differs, count does not.
	live = live[:7]   // The final window is missing entirely.
	return layoutDrift(saved, live)
}

func TestRestoreOutputLayoutWarning(t *testing.T) {
	old := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(old) })
	for _, lang := range []string{"en", "zh"} {
		i18n.SetLang(lang)
		out := renderLayoutDrift(21, outputDrift(), 80)
		for _, want := range []string{"21", "8", "example-0:0", "4 → 2", i18n.Tr("arrangement changed", "排布不同"), i18n.Tr("3 more windows", "另有 3 个窗口"), restoreTraceHint} {
			if !strings.Contains(out, want) {
				t.Fatalf("%s: warning lost %q:\n%s", lang, want, out)
			}
		}
		if strings.Contains(out, "example-5:0") || strings.Contains(out, "saved-geometry") || strings.Contains(out, "default arrangement") {
			t.Fatalf("verbose or unbounded warning:\n%s", out)
		}
		if strings.Count(out, restoreTraceHint) != 1 {
			t.Fatal("print one full-log command")
		}
		// The copyable log command deliberately remains intact below its own label.
		narrow := renderLayoutDrift(21, outputDrift(), 40)
		if !strings.Contains(narrow, restoreTraceHint) {
			t.Fatal("narrow output truncated the log command")
		}
		missing := renderLayoutDrift(1, outputDrift()[7:], 80)
		if !strings.Contains(missing, i18n.Tr("window missing", "窗口未恢复")) {
			t.Fatal("missing window must not be described as a count change")
		}
		if renderLayoutDrift(21, nil, 80) != "" {
			t.Fatal("faithful restore must not print a warning")
		}
	}
}

func TestRestoreOutputFullLogSurvivesCompactWarning(t *testing.T) {
	old := i18n.Lang()
	i18n.SetLang("en")
	t.Cleanup(func() { i18n.SetLang(old) })
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", "")
	var lines [][]string
	var live strings.Builder
	for i := 0; i < 23; i++ {
		name := fmt.Sprintf("example-%d", i)
		lines = append(lines, shapeWin(name, "0", "1234,80x24,0,0,1"), shapePane(name, "0", "0"), shapePane(name, "0", "1"))
		fmt.Fprintf(&live, "%s\t0\t1\t1234,80x24,0,0,1\n", name)
	}
	t.Setenv("RESTORE_TEST_WINDOWS", live.String())
	useDoctorTmuxStub(t, "printf '%s' \"$RESTORE_TEST_WINDOWS\"\n")
	save := writeShapeSave(t, lines...)
	reportLayoutDrift(save)
	data, err := os.ReadFile(diag.DayFile(state.LogsDir(), time.Now().Format("2006-01-02")))
	if err != nil {
		t.Fatal(err)
	}
	entries := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(entries) != 24 {
		t.Fatalf("want a summary and 23 diagnoses, got %d", len(entries))
	}
	for i, raw := range entries[1:] {
		var entry diag.Entry
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			t.Fatal(err)
		}
		if entry.Event != "restore.layout.window" || !strings.Contains(entry.Msg, fmt.Sprintf("example-%d:0", i)) || !strings.Contains(entry.Msg, "tmux refuses") || entry.Truncated {
			t.Fatalf("compact display lost full diagnosis in real log: %+v", entry)
		}
	}
}
