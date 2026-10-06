package radar

import (
	"crypto/sha256"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// homeTree is every file under HOME with its content hash and modification time.
func homeTree(t *testing.T, home string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(home, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(home, p)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			out[rel+"/"] = "dir"
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		out[rel] = string(sum[:]) + info.ModTime().String()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The pane browser's read SHALL have no side effects. It went through GatherAgents, whose
// status work, native records, bindings and orphan sweep write and delete under HOME: a
// /api/panes read deleted waiting/%909 and watched/%909 (%12, 2026-10-06). Here HOME holds
// what that machinery would touch, the panes include one only the process tree identifies
// (which GatherAgents samples, writing frame and CPU baselines) and one with a built-in
// icon not yet cached, and a read must leave every file as it was, while saying the same
// thing about each pane as the radar does.
func TestPanesReadWritesNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, p := range []string{state.WaitingPath("%909"), state.WatchedPath("%909"), state.ActivePath("%909")} {
		if err := state.WriteMarker(p, "x"); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.Touch(state.FinishedPath("%909")); err != nil {
		t.Fatal(err)
	}
	// A native record nobody can check, past the grace: native.Live would remove it.
	if err := native.Save(native.Record{Agent: "claude", SessionID: "stale", UpdatedAt: time.Now().Unix() - 13*3600}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(builtinIconCachePath("codex")); !os.IsNotExist(err) {
		t.Fatalf("the codex icon is already cached (%v); the cold case needs it absent", err)
	}

	radarLines := []string{
		paneLine("%91", "w", "0", "0", "✳ ready", "claude", 1700000000, 900091, "/tmp/nope"),
		paneLine("%92", "w", "0", "1", "", "bash", 1700000000, 900092, "/tmp/nope"),
		paneLine("%93", "w", "0", "2", "", "node", 1700000000, 900093, "/tmp/nope"),
	}
	rowLines := []string{
		paneRowLine("%91", "w", "0", "0", "/tmp/nope", "claude", "✳ ready", "1", "0"),
		paneRowLine("%92", "w", "0", "1", "/tmp/nope", "bash", "", "0", "0"),
		paneRowLine("%93", "w", "0", "2", "/tmp/nope", "node", "", "0", "0"),
	}
	procs := map[int]procInfo{
		900093: {ppid: 1, command: "node /usr/local/lib/node_modules/@openai/codex/bin/codex"},
	}
	before := homeTree(t, home)

	var rows []PaneRow
	withFixture(t, radarLines, func() {
		procSnapshot = func() map[int]procInfo { return procs }
		origPanes, origViews := panesSource, clientViewSource
		panesSource = func() []string { return rowLines }
		clientViewSource = func() []string { return nil }
		defer func() { panesSource, clientViewSource = origPanes, origViews }()
		b, err := PanesJSONBytes()
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &rows); err != nil {
			t.Fatal(err)
		}
	})

	after := homeTree(t, home)
	for p, v := range before {
		if after[p] != v {
			t.Errorf("the read changed or removed %s", p)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			t.Errorf("the read created %s", p)
		}
	}

	got := map[string]PaneRow{}
	for _, r := range rows {
		got[r.PaneID] = r
	}
	if r := got["%91"]; r.Tier != "agent" || r.Agent != "Claude Code" {
		t.Errorf("%%91 = %+v, want the Claude Code agent", r)
	}
	if r := got["%92"]; r.Tier != "plain" || r.Agent != "" {
		t.Errorf("%%92 = %+v, want a plain pane", r)
	}
	if r := got["%93"]; r.Tier != "agent" || r.Agent != "Codex" || r.Icon != builtinIconCachePath("codex") {
		t.Errorf("%%93 = %+v, want the Codex agent with its icon path", r)
	}

	// The radar, on the same panes, names the same agents with the same icons and roles.
	withFixture(t, radarLines, func() {
		procSnapshot = func() map[int]procInfo { return procs }
		for _, p := range GatherAgents() {
			if p.source != "tmux" || p.Watched {
				continue
			}
			r := got[p.PaneID]
			if r.Tier != "agent" || r.Agent != p.Agent || r.Icon != p.icon || r.Role != p.Role() {
				t.Errorf("radar %s = %s/%s/%s, browser = %+v", p.PaneID, p.Agent, p.icon, p.Role(), r)
			}
		}
	})
}

// identifiedAgentPanes gives the supervisor role exactly as the radar does: the stamped
// pane holds it, and a cwd in the HQ home counts only when no pane is stamped.
func TestIdentifiedAgentPanesRole(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	hq := state.HQHome()
	stamped := strings.Join([]string{"%1", "hq", "0", "0", "✳ ready", "claude", "0", "0", "1", "/tmp/elsewhere", "0", hq}, "\t")
	parked := paneLine("%2", "w", "0", "0", "✳ ready", "claude", 0, 2, hq)
	roles := func(lines ...string) map[string]string {
		out := map[string]string{}
		withFixture(t, lines, func() {
			for _, p := range identifiedAgentPanes() {
				out[p.PaneID] = p.Role()
			}
		})
		return out
	}
	if r := roles(stamped, parked); r["%1"] != "supervisor" || r["%2"] != "" {
		t.Errorf("with a stamp: %v, want only the stamped pane", r)
	}
	if r := roles(parked); r["%2"] != "supervisor" {
		t.Errorf("no stamp: %v, want the pane in the HQ home", r)
	}
}
