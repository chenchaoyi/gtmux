package radar

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func staleCodexFixture(t *testing.T, corrupt bool, activeAt, finishedAt time.Time) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", "")
	const pane, loc = "%17", "synthetic:0.0"
	r := resume.Record{Agent: "codex", SessionID: "thread", Cwd: "/work", Frontend: "0123456789abcdef0123456789abcdef"}
	if err := resume.Save(loc, r); err != nil {
		t.Fatal(err)
	}
	if corrupt {
		path := filepath.Join(resume.Dir(), base64.RawURLEncoding.EncodeToString([]byte(loc))+".json")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, []byte("residue!")...), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.WriteMarker(state.ActivePath(pane), "thread"); err != nil {
		t.Fatal(err)
	}
	if err := state.WriteMarker(state.TurnIDPath(pane), "old-turn"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(state.ActivePath(pane), activeAt, activeAt); err != nil {
		t.Fatal(err)
	}
	if !finishedAt.IsZero() {
		if err := state.Touch(state.FinishedPath(pane)); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(state.FinishedPath(pane), finishedAt, finishedAt); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.WriteLastFinished(pane); err != nil {
		t.Fatal(err)
	}
	return pane
}

func TestCSR001StaleActiveCannotReviveReadyFinishedPane(t *testing.T) {
	for _, tc := range []struct {
		name, prefix, want                                             string
		corrupt, fresh, noFinish, olderFinish, futureFinish, wait, cpu bool
	}{
		{name: "missing frontend", want: "idle"},
		{name: "corrupt resume", corrupt: true, want: "idle"},
		{name: "visible work", prefix: "• Working (3s • esc to interrupt)\n", want: "working"},
		{name: "queued", prefix: "Messages to be submitted after next tool call\n", want: "working"},
		{name: "startup", prefix: "• Starting MCP servers (1/4): tools (0s • esc to interrupt)\n", want: "working"},
		{name: "fresh start", fresh: true, want: "working"},
		{name: "no finished evidence", noFinish: true, want: "working"},
		{name: "older finished", olderFinish: true, want: "working"},
		{name: "future finished", futureFinish: true, want: "working"},
		{name: "approval", wait: true, want: "waiting"},
		{name: "cpu work", cpu: true, want: "working"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().Truncate(time.Second)
			active, finished := now.Add(-time.Hour), now.Add(-30*time.Minute)
			if tc.fresh {
				active = now
			}
			if tc.noFinish {
				finished = time.Time{}
			}
			if tc.olderFinish {
				finished = now.Add(-2 * time.Hour)
			}
			if tc.futureFinish {
				finished = now.Add(time.Hour)
			}
			pane := staleCodexFixture(t, tc.corrupt, active, finished)
			frame := tc.prefix + "› Ask Codex to do anything\n GPT-6 · /work"
			if tc.wait {
				if err := state.WriteMarker(state.WaitingPath(pane), "permission"); err != nil {
					t.Fatal(err)
				}
				frame = "Would you like to run the following command?\n\n$ touch /synthetic\n\n› 1. Yes, proceed (y)\n  2. No, and tell Codex what to do differently (esc)"
			}
			paths := []string{state.ActivePath(pane), state.TurnIDPath(pane), state.FinishedPath(pane), state.WaitingPath(pane)}
			before := map[string][]byte{}
			stamps := map[string]time.Time{}
			for _, path := range paths {
				if b, err := os.ReadFile(path); err == nil {
					before[path] = b
					info, _ := os.Stat(path)
					stamps[path] = info.ModTime()
				}
			}
			capture := paneCapture
			t.Cleanup(func() { paneCapture = capture })
			paneCapture = func(string) string { return frame }
			withFixture(t, []string{paneLine(pane, "synthetic", "0", "0", "Codex", "codex", now.Unix(), 900017, "/work")}, func() {
				if tc.cpu {
					state.PaneCPUWorking(pane, 0, now.Unix()-2)
					procSnapshot = func() map[int]procInfo { return map[int]procInfo{900017: {cpu: 20, command: "codex"}} }
				}
				for i := 0; i < 2; i++ {
					rows := GatherPaneAgents(pane)
					if len(rows) != 1 || rows[0].Status != tc.want {
						t.Fatalf("stale-active display: want %s got %+v", tc.want, rows)
					}
					if tc.want == "idle" && (rows[0].Since != finished.Unix() || rows[0].Confirmed || rows[0].Latest || rows[0].CompletionID != "" || rows[0].CompletedAt != 0) {
						t.Fatalf("display minted or re-dated completion: %+v", rows[0])
					}
				}
			})
			for _, path := range paths {
				b, err := os.ReadFile(path)
				old, exists := before[path]
				if !exists {
					if !os.IsNotExist(err) {
						t.Fatalf("display created %s", path)
					}
					continue
				}
				info, statErr := os.Stat(path)
				if err != nil || statErr != nil || string(b) != string(old) || !info.ModTime().Equal(stamps[path]) {
					t.Fatalf("display changed lifecycle marker %s", path)
				}
			}
		})
	}
}

func TestCSR002SupersededActiveDoesNotDateCurrentWork(t *testing.T) {
	for _, elapsed := range []string{"12s", "1m 2s", "unknown"} {
		t.Run(elapsed, func(t *testing.T) {
			now := time.Now().Truncate(time.Second)
			pane := staleCodexFixture(t, true, now.Add(-17*time.Hour), now.Add(-time.Hour))
			frame := "• Working (" + elapsed + " • esc to interrupt)\n\n› Ask Codex to do anything\n GPT-6 · /work"
			capture := paneCapture
			t.Cleanup(func() { paneCapture = capture })
			paneCapture = func(string) string { return frame }
			withFixture(t, []string{paneLine(pane, "synthetic", "0", "0", "Codex", "codex", now.Unix(), 900017, "/work")}, func() {
				rows := GatherPaneAgents(pane)
				if len(rows) != 1 || rows[0].Status != "working" {
					t.Fatalf("actual work disappeared: %+v", rows)
				}
				b, err := agentsJSONBytes(rows)
				if err != nil {
					t.Fatal(err)
				}
				var wire []map[string]any
				if err := json.Unmarshal(b, &wire); err != nil {
					t.Fatal(err)
				}
				if elapsed == "unknown" {
					if rows[0].Since != 0 || wire[0]["since_uncertain"] != true {
						t.Fatalf("unknown duration inherited obsolete start: %s", b)
					}
				} else {
					want := int64(12)
					if elapsed == "1m 2s" {
						want = 62
					}
					got := time.Now().Unix() - rows[0].Since
					if got < want || got > want+2 || wire[0]["since_uncertain"] == true {
						t.Fatalf("current elapsed %s used stale start: since=%d age=%d wire=%s", elapsed, rows[0].Since, got, b)
					}
				}
			})
		})
	}
}
