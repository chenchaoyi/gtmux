package radar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// paneRowLine builds a panesSource field-line: id, session, window, pane, cwd,
// command, title, active, in_mode.
func paneRowLine(id, session, win, pane, cwd, cmd, title, active, inMode string) string {
	return strings.Join([]string{id, session, win, pane, cwd, cmd, title, active, inMode}, "\t")
}

func withPaneFixture(t *testing.T, lines []string, agents map[string]string, fn func()) {
	t.Helper()
	origPanes, origAgents := panesSource, agentPaneSet
	panesSource = func() []string { return lines }
	agentPaneSet = func() (map[string]string, map[string]bool, map[string]string) {
		set := map[string]bool{}
		icons := map[string]string{}
		for id, name := range agents {
			set[id] = true
			icons[id] = "/Applications/" + name + ".app" // stub official-icon hint
		}
		return agents, set, icons
	}
	defer func() { panesSource, agentPaneSet = origPanes, origAgents }()
	fn()
}

func TestGatherPanes_Tiers(t *testing.T) {
	lines := []string{
		paneRowLine("%1", "MP", "0", "0", "/p/mp", "claude", "✳ build the thing", "1", "0"),
		paneRowLine("%2", "MP", "0", "1", "/p/mp", "bash", "", "0", "0"),
		paneRowLine("%3", "MP", "1", "0", "/p/mp", "vim", "editing", "1", "1"),
	}
	// The radar classifies %1 as an agent (Claude Code); %2 and %3 are plain.
	agents := map[string]string{"%1": "Claude Code"}

	var rows []PaneRow
	withPaneFixture(t, lines, agents, func() { rows = GatherPanes() })

	if len(rows) != 3 {
		t.Fatalf("want 3 panes, got %d", len(rows))
	}
	byID := map[string]PaneRow{}
	for _, r := range rows {
		byID[r.PaneID] = r
	}
	if byID["%1"].Tier != "agent" || byID["%1"].Agent != "Claude Code" {
		t.Fatalf("%%1 should be an agent pane: %+v", byID["%1"])
	}
	// An agent pane carries the official-icon hint (so the pane browser shows the real
	// logo, not the monogram); a plain pane never does.
	if byID["%1"].Icon == "" {
		t.Fatalf("%%1 (agent) should carry an icon hint: %+v", byID["%1"])
	}
	if byID["%2"].Tier != "plain" || byID["%2"].Agent != "" || byID["%2"].Icon != "" {
		t.Fatalf("%%2 (bare shell) should be plain with no icon: %+v", byID["%2"])
	}
	if byID["%3"].Tier != "plain" {
		t.Fatalf("%%3 (vim) is a plain pane, not an agent: %+v", byID["%3"])
	}
	// Field fidelity: loc, cwd, active, in_mode, title.
	if byID["%1"].Loc != "MP:0.0" || byID["%1"].Cwd != "/p/mp" || !byID["%1"].Active {
		t.Fatalf("%%1 fields wrong: %+v", byID["%1"])
	}
	if !byID["%3"].InMode {
		t.Fatalf("%%3 is in copy-mode; in_mode should be true: %+v", byID["%3"])
	}
}

func TestPanesJSONBytes_AlwaysArray(t *testing.T) {
	withPaneFixture(t, nil, nil, func() {
		b, err := PanesJSONBytes()
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "[]" {
			t.Fatalf("empty pane set → [], got %q", b)
		}
		var rows []PaneRow
		if json.Unmarshal(b, &rows) != nil {
			t.Fatal("output must decode as a JSON array")
		}
	})
}

// Every tier carries the git identity of its cwd, not just agent rows. A surface reads
// `branch` to know the pane HAS a repo at all — the phone offers its Diff control only
// then, because `GET /api/diff` returns "" for a non-repo cwd and an ungated button
// opened an empty sheet. Plain panes are exactly where someone does git by hand, so
// this is the tier that most needed the answer and was the one not being asked.
func TestGatherPanes_GitIdentityOnEveryTier(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/trunk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bare := t.TempDir() // no .git anywhere above it

	orig, origAgents := panesSource, agentPaneSet
	defer func() { panesSource, agentPaneSet = orig, origAgents }()
	panesSource = func() []string {
		return []string{
			"%1\ts\t0\t0\t" + repo + "\tbash\t\t1\t0",
			"%2\ts\t0\t1\t" + bare + "\tbash\t\t0\t0",
		}
	}
	agentPaneSet = func() (map[string]string, map[string]bool, map[string]string) {
		return map[string]string{}, map[string]bool{}, map[string]string{}
	}

	rows := GatherPanes()
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Tier != "plain" || rows[0].Branch != "trunk" || rows[0].Project != filepath.Base(repo) {
		t.Errorf("a plain pane in a repo must carry its git identity, got %+v", rows[0])
	}
	// …and a pane outside a repo says so with empty fields, which is what hides the control.
	if rows[1].Branch != "" || rows[1].Project != "" {
		t.Errorf("a pane outside a repo must carry no git identity, got %+v", rows[1])
	}
}

// paneRowLineW builds a field-line WITH the window id and name — the tmux-id-surface
// fields, appended at the end of the format so old lines keep parsing.
func paneRowLineW(id, session, win, pane, cwd, cmd, title, active, inMode, winID, winName string) string {
	return strings.Join([]string{id, session, win, pane, cwd, cmd, title, active, inMode, winID, winName}, "\t")
}

// The WINDOW needs a stable anchor, and the index cannot be it. Measured on a real fleet:
// 8 of 12 active windows had index 0, so every window would label and group as "…:0".
// `@N` is unique per server; the NAME is a gloss that drifts with `automatic-rename`.
func TestGatherPanes_CarriesWindowIdentity(t *testing.T) {
	lines := []string{
		paneRowLineW("%1", "MP", "0", "0", "/p/mp", "claude", "✳ x", "1", "0", "@7", "multipilot"),
		paneRowLineW("%2", "MP", "0", "1", "/p/mp", "bash", "", "0", "0", "@7", "multipilot"),
		// A SECOND window whose index is also 0 — the case that breaks index-based
		// identity, and the reason @N exists.
		paneRowLineW("%9", "Pica", "0", "0", "/p/pica", "zsh", "", "1", "0", "@9", "sat-monitor"),
	}
	withPaneFixture(t, lines, map[string]string{"%1": "Claude Code"}, func() {
		rows := GatherPanes()
		if len(rows) != 3 {
			t.Fatalf("rows = %d, want 3", len(rows))
		}
		if rows[0].WinID != "@7" || rows[0].WinName != "multipilot" {
			t.Errorf("row0 win = %q/%q, want @7/multipilot", rows[0].WinID, rows[0].WinName)
		}
		// Two panes of ONE window share the id — that is what makes grouping possible.
		if rows[1].WinID != rows[0].WinID {
			t.Errorf("panes of the same window must share win_id: %q vs %q", rows[1].WinID, rows[0].WinID)
		}
		// Same window INDEX, different window — distinguishable only by the id.
		if rows[2].Window != rows[0].Window {
			t.Fatalf("fixture should give both windows index %q", rows[0].Window)
		}
		if rows[2].WinID == rows[0].WinID {
			t.Error("two different windows sharing index 0 must NOT share win_id")
		}
	})
}

// The fields are ADDITIVE: a line without them still parses, so nothing depends on a
// tmux that reports them.
func TestGatherPanes_WindowIdentityIsOptional(t *testing.T) {
	lines := []string{paneRowLine("%1", "MP", "0", "0", "/p/mp", "bash", "", "1", "0")}
	withPaneFixture(t, lines, nil, func() {
		rows := GatherPanes()
		if len(rows) != 1 {
			t.Fatalf("rows = %d, want 1", len(rows))
		}
		if rows[0].WinID != "" || rows[0].WinName != "" {
			t.Errorf("absent fields must stay empty, got %q/%q", rows[0].WinID, rows[0].WinName)
		}
		// omitempty keeps them out of the payload entirely.
		b, _ := json.Marshal(rows[0])
		if strings.Contains(string(b), "win_id") {
			t.Errorf("empty win_id must be omitted: %s", b)
		}
	})
}

// The name must NOT be mistaken for an identity: tmux lets two windows share a name.
func TestGatherPanes_WindowNameIsNotAnIdentity(t *testing.T) {
	lines := []string{
		paneRowLineW("%1", "A", "0", "0", "/p", "bash", "", "1", "0", "@1", "dev"),
		paneRowLineW("%2", "B", "0", "0", "/p", "bash", "", "1", "0", "@2", "dev"),
	}
	withPaneFixture(t, lines, nil, func() {
		rows := GatherPanes()
		if rows[0].WinName != rows[1].WinName {
			t.Fatal("fixture should give both windows the same name")
		}
		if rows[0].WinID == rows[1].WinID {
			t.Error("same name, different windows — the ids must differ")
		}
	})
}

// A shell writes the HOSTNAME into every pane's title, so on a real fleet four different
// panes all read `ccy-MBP2024-M4-Office.local` — a label that distinguishes nothing, while
// a sibling shell with no title read `bash`, which is at least true. tmux-id-surface says
// it outright: pane_title is not a usable per-pane name.
//
// Dropped in the CORE because only this machine knows its own hostname; the phone and the
// web are clients and cannot check it.
func TestMeaningfulTitle_DropsTheHostname(t *testing.T) {
	orig := hostTitle
	defer func() { hostTitle = orig }()
	hostTitle = "ccy-MBP2024-M4-Office.local"

	for _, in := range []string{"ccy-MBP2024-M4-Office.local", "  ccy-MBP2024-M4-Office.local  ", "ccy-MBP2024-M4-Office"} {
		if got := meaningfulTitle(in); got != "" {
			t.Errorf("meaningfulTitle(%q) = %q, want dropped — a hostname names no pane", in, got)
		}
	}
	// A REAL title survives: tolerance must not become erasure.
	for _, in := range []string{"make check", "logs", "✳ build the thing", "ccy-MBP2024-M4-Office.local — extra"} {
		if got := meaningfulTitle(in); got != strings.TrimSpace(in) {
			t.Errorf("meaningfulTitle(%q) = %q, want it kept", in, got)
		}
	}
	// No hostname resolvable → nothing is dropped, rather than guessing.
	hostTitle = ""
	if got := meaningfulTitle("ccy-MBP2024-M4-Office.local"); got == "" {
		t.Error("with no hostname known, a title must be left alone")
	}
}

// …and the same thing at the ROW level, because a correct predicate that nothing calls is
// still a pane labelled with a hostname. (The pure-function test above passes even when
// GatherPanes never invokes it — verified.)
func TestGatherPanes_DropsHostnameTitle(t *testing.T) {
	orig := hostTitle
	defer func() { hostTitle = orig }()
	hostTitle = "my-mac.local"

	lines := []string{
		paneRowLine("%1", "S", "0", "0", "/p", "bash", "my-mac.local", "1", "0"),
		paneRowLine("%2", "S", "0", "1", "/p", "tail", "deploy.log", "0", "0"),
	}
	withPaneFixture(t, lines, nil, func() {
		rows := GatherPanes()
		if rows[0].Title != "" {
			t.Errorf("row %q kept a hostname title %q — the surfaces will render it", rows[0].PaneID, rows[0].Title)
		}
		if rows[1].Title != "deploy.log" {
			t.Errorf("row %q lost a real title: %q", rows[1].PaneID, rows[1].Title)
		}
	})
}
