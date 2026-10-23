package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// A resurrect save line for one pane that was RUNNING AN AGENT at save time (the
// plan lists a conversation only for those — see savedPane.evidence).
func savePane(session, win, pane string) string {
	return savePaneCmd(session, win, pane, "/some/dir", "2.1.220", ":claude")
}

// A resurrect save line for a pane that was sitting at a bare shell.
func saveShellPane(session, win, pane, dir string) string {
	return savePaneCmd(session, win, pane, dir, "bash", ":")
}

func savePaneCmd(session, win, pane, dir, cmd, full string) string {
	return "pane\t" + session + "\t" + win + "\t1\t:\t" + pane + "\t✳ title\t:" + dir + "\t1\t" + cmd + "\t123\t" + full
}

func TestBuildRestorePlan(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Save: session "MP" has windows 0 (panes 0,1) and 1 (pane 0) = 2 windows / 3
	// panes; session "solo" has one shell pane.
	save := filepath.Join(t.TempDir(), "resurrect.txt")
	body := savePane("MP", "0", "0") + "\n" +
		savePane("MP", "0", "1") + "\n" +
		savePane("MP", "1", "0") + "\n" +
		savePane("solo", "0", "0") + "\n" +
		"window\tMP\t0\t:MP\t...\n" // a non-pane line is ignored
	if err := os.WriteFile(save, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	// Resume records: two live agents at MP:0.0 and MP:1.0, plus a STALE MP:0.2
	// whose pane no longer exists in the save — it must be dropped.
	_ = resume.Save("MP:0.0", resume.Record{Agent: "claude", SessionID: "s-00", Cwd: "/p/mp", UpdatedAt: 300})
	_ = resume.Save("MP:1.0", resume.Record{Agent: "codex", SessionID: "s-10", Cwd: "/p/mp", UpdatedAt: 200})
	_ = resume.Save("MP:0.2", resume.Record{Agent: "claude", SessionID: "s-stale", Cwd: "/p/mp", UpdatedAt: 100})

	plan := buildRestorePlanFrom(save)

	if len(plan.Sessions) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(plan.Sessions))
	}
	mp := plan.Sessions[0] // first appearance order
	if mp.Name != "MP" || mp.Windows != 2 || mp.Panes != 3 {
		t.Fatalf("MP counts wrong: %+v", mp)
	}
	// The stale MP:0.2 record must NOT appear — only the two panes still in the save.
	if len(mp.Agents) != 2 {
		t.Fatalf("MP should list 2 agents (stale MP:0.2 dropped), got %d: %+v", len(mp.Agents), mp.Agents)
	}
	ids := map[string]bool{}
	for _, a := range mp.Agents {
		ids[a.SessionID] = true
	}
	if ids["s-stale"] {
		t.Fatal("the stale locator's conversation must not be in the plan")
	}
	if !ids["s-00"] || !ids["s-10"] {
		t.Fatalf("both live agents should be present, got %v", ids)
	}

	if solo := plan.Sessions[1]; solo.Name != "solo" || len(solo.Agents) != 0 {
		t.Fatalf("solo should be an agent-less session: %+v", solo)
	}

	if plan.agentCount() != 2 {
		t.Fatalf("agentCount = %d, want 2", plan.agentCount())
	}
	// s-00 is claude with NO transcript on disk → dead (×); s-10 is codex → always
	// resumable (we can't inspect its store). So the headline should count 1 resumable
	// and note 1 with-no-transcript, matching the eventual "resumed 1".
	if got := plan.resumableCount(); got != 1 {
		t.Fatalf("resumableCount = %d, want 1 (only the codex agent is resumable)", got)
	}
	if got := plan.deadCount(); got != 1 {
		t.Fatalf("deadCount = %d, want 1 (the claude agent has no transcript)", got)
	}
}

// The plan is a promise about what restore will do, so it must apply the SAME
// liveness gate: a pane that was a bare shell when the layout was saved gets no
// conversation listed, however well-remembered its locator is. (Before this, the
// plan happily advertised the phantom conversations the resume path then injected.)
func TestBuildRestorePlan_shellPaneIsNotPromisedAnAgent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	save := filepath.Join(t.TempDir(), "resurrect.txt")
	body := saveShellPane("日常更新", "0", "0", "/Users/x") + "\n" +
		savePane("docs", "1", "0") + "\n"
	if err := os.WriteFile(save, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// Both panes have a record; only the one the save shows running an agent counts.
	_ = resume.Save("日常更新:0.0", resume.Record{Agent: "codex", SessionID: "phantom", Cwd: "/Users/x", UpdatedAt: 300})
	_ = resume.Save("docs:1.0", resume.Record{Agent: "codex", SessionID: "real", Cwd: "/some/dir", UpdatedAt: 300})

	plan := buildRestorePlanFrom(save)
	for _, s := range plan.Sessions {
		for _, a := range s.Agents {
			if a.SessionID == "phantom" {
				t.Fatalf("a pane the save shows at a shell must not be listed: %+v", a)
			}
		}
	}
	if plan.agentCount() != 1 {
		t.Fatalf("only the live agent pane should be listed, got %d: %+v", plan.agentCount(), plan.Sessions)
	}
}

// The other half of the same evidence: a pane that WAS running an agent but whose
// resume record is missing recovers its conversation from the save's own command
// line, instead of coming back as an empty shell.
func TestBuildRestorePlan_recoversIDFromTheSavedCommand(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	save := filepath.Join(t.TempDir(), "resurrect.txt")
	line := savePaneCmd("gtmux dev", "0", "0", "/p/gtmux", "2.1.220", ":claude --resume affb005b")
	if err := os.WriteFile(save, []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := buildRestorePlanFrom(save) // no resume records at all
	if plan.agentCount() != 1 {
		t.Fatalf("want the conversation recovered from the saved command line, got %+v", plan.Sessions)
	}
	if a := plan.Sessions[0].Agents[0]; a.SessionID != "affb005b" || a.Agent != "claude" {
		t.Fatalf("agent = %+v", a)
	}
}

func TestBuildRestorePlan_NoSave(t *testing.T) {
	if p := buildRestorePlanFrom(""); len(p.Sessions) != 0 || p.SavePath != "" {
		t.Fatalf("empty save → empty plan, got %+v", p)
	}
}

func TestLocSession(t *testing.T) {
	cases := map[string]string{
		"MP:0.1":        "MP",
		"a:b:2.3":       "a:b", // colon in session name → split on LAST colon
		"gtmux dev:1.0": "gtmux dev",
	}
	for in, want := range cases {
		if got := locSession(in); got != want {
			t.Errorf("locSession(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRestorePlanFallbackOwnership(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		ownerPresent, ambiguous bool
		want                    int
	}{
		{name: "original owner remains a shell", ownerPresent: true, want: 0},
		{name: "ambiguous history", ambiguous: true, want: 0},
		{name: "unique rename", want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			save := filepath.Join(t.TempDir(), "resurrect.txt")
			body := savePaneCmd("worker", "0", "0", "/project", "codex", ":codex") + "\n"
			if tc.ownerPresent {
				body += saveShellPane("old", "0", "0", "/project") + "\n"
			}
			if err := os.WriteFile(save, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if err := resume.Save("old:0.0", resume.Record{Agent: "codex", SessionID: "a", Cwd: "/project"}); err != nil {
				t.Fatal(err)
			}
			if tc.ambiguous {
				if err := resume.Save("other:0.0", resume.Record{Agent: "codex", SessionID: "b", Cwd: "/project"}); err != nil {
					t.Fatal(err)
				}
			}
			if plan := buildRestorePlanFrom(save); plan.agentCount() != tc.want {
				t.Fatalf("plan promises wrong conversation count: %+v", plan)
			}
		})
	}
}

// These previews never start a server or touch a pane. The query seam is also an
// assertion that availability is one topology read, independent of agent history.
func TestPendingRestorePlan(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	save := filepath.Join(t.TempDir(), "last")
	if err := os.WriteFile(save, []byte(saveShellPane("attached", "0", "0", "/fixture")+"\n"+saveShellPane("missing", "0", "0", "/fixture")+"\n"+saveShellPane("detached shell", "0", "0", "/fixture")), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, output string
		queryErr     error
		want         []string
		wantErr      bool
	}{
		{"no server after reboot", "", &exec.ExitError{Stderr: []byte("no server running on /fixture/socket")}, []string{"attached", "missing", "detached shell"}, false},
		{"partially restored", "1\t@1\t%1\tattached\n0\t@2\t%2\tdetached shell\n0\t@2\t%3\tdetached shell\n0\t@3\t%4\tdetached shell\n0\t@4\t%5\tnew shell", nil, []string{"detached shell", "new shell", "missing"}, false},
		{"all attached", "1\t@1\t%1\tattached\n2\t@2\t%2\tmissing\n1\t@3\t%3\tdetached shell", nil, nil, false},
		{"permission failure is unknown", "", &exec.ExitError{Stderr: []byte("error connecting to /fixture/socket (Permission denied)")}, nil, true},
		{"missing binary is unknown", "", exec.ErrNotFound, nil, true},
		{"bad output is unknown", "unexpected", nil, nil, true},
		{"bad attached count is unknown", "x\t@1\t%1\tattached", nil, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			plan, err := pendingRestorePlan(save, func(args ...string) (string, error) {
				calls++
				if len(args) != 4 || args[0] != "list-panes" || args[1] != "-a" || args[2] != "-F" {
					t.Fatalf("unexpected command: %v", args)
				}
				return tc.output, tc.queryErr
			})
			if (err != nil) != tc.wantErr || calls != 1 {
				t.Fatalf("err=%v calls=%d", err, calls)
			}
			var names []string
			for _, s := range plan.Sessions {
				names = append(names, s.Name)
			}
			if !reflect.DeepEqual(names, tc.want) {
				t.Fatalf("sessions=%v, want %v", names, tc.want)
			}
			if tc.name == "partially restored" {
				s := plan.Sessions[0]
				if s.State != "detached" || s.Windows != 2 || s.Panes != 3 || len(s.Agents) != 0 {
					t.Fatalf("live counts / no stale resume promises: %+v", s)
				}
				if plan.Sessions[2].State != "missing" {
					t.Fatal("missing session loses its state")
				}
			}
		})
	}
}

func TestPendingRestorePlanWithoutSave(t *testing.T) {
	plan, err := pendingRestorePlan("", func(...string) (string, error) { return "0\t@1\t%1\tlive shell", nil })
	if err != nil || len(plan.Sessions) != 1 || plan.Sessions[0].Name != "live shell" {
		t.Fatalf("detached unsaved session: %+v %v", plan, err)
	}
	_, err = pendingRestorePlan("", func(...string) (string, error) { return "", errors.New("unavailable") })
	if err == nil {
		t.Fatal("failed query must not be known empty")
	}
}

func TestPendingRestoreCLIIsReadOnlyEvenInsideTmux(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TMUX", "synthetic")
	dir := t.TempDir()
	trace := filepath.Join(dir, "calls")
	t.Setenv("GTMUX_TEST_RESTORE_TRACE", trace)
	bin := filepath.Join(dir, "tmux")
	script := `#!/bin/sh
[ "$1" = "-u" ] && shift
printf '%s\n' "$1" >> "$GTMUX_TEST_RESTORE_TRACE"
[ "$1" = "list-panes" ] || exit 77
printf '0\t@1\t%%1\tlive shell\n'
`
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	old := tmux.Bin
	tmux.Bin = bin
	t.Cleanup(func() { tmux.Bin = old })
	var rc int
	out := captureStdout(t, func() { rc = cmdRestore([]string{"--plan", "--pending", "--json"}) })
	if rc != 0 || !strings.Contains(out, `"name":"live shell"`) || !strings.Contains(out, `"state":"detached"`) {
		t.Fatalf("rc=%d out=%s", rc, out)
	}
	if cmdRestore([]string{"--pending"}) != 1 {
		t.Fatal("standalone pending must not trigger restore")
	}
	calls, err := os.ReadFile(trace)
	if err != nil || string(calls) != "list-panes\n" {
		t.Fatalf("preview issued a mutating command: %q %v", calls, err)
	}
}
