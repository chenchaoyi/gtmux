package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/launchhost"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestCPR005DetachedCodexStopUsesOnlyCurrentPaneBinding(t *testing.T) {
	for _, phase := range []string{"conflicting index", "expired receipt", "current binding"} {
		for _, inherited := range []string{"", "%20"} {
			t.Run(phase+"/inherited="+inherited, func(t *testing.T) {
				hermeticEnv(t)
				t.Setenv("CODEX_HOME", t.TempDir())
				t.Setenv("TMUX_PANE", inherited)
				t.Setenv("GTMUX_HQ", "")
				const owner, peer, root, other, loc = "%19", "%20", "root-thread", "other-thread", "dev:0.0"
				cwd, homeID := t.TempDir(), transcript.CodexHomeID()
				stub := filepath.Join(t.TempDir(), "tmux")
				script := `#!/bin/sh
case "$2" in
list-panes)
  printf '%%19\tcodex\t%s\tdev:0.0\n%%20\tcodex\t%s\tdev:0.1\n' "$FAKE_CODEX_PANE_CWD" "$FAKE_CODEX_PANE_CWD"
  ;;
display-message)
  [ "$5" = '%19' ] || exit 0
  case "$6" in
  '#{pane_id}'*) printf '%%19\t100\tdev:0.0\n' ;;
  '#{session_name}:#{window_index}.#{pane_index}') printf 'dev:0.0\n' ;;
  '#{session_name}') printf 'dev\n' ;;
  '#{pane_current_path}') printf '%s\n' "$FAKE_CODEX_PANE_CWD" ;;
  esac
  ;;
esac
`
				t.Setenv("FAKE_CODEX_PANE_CWD", cwd)
				if err := os.WriteFile(stub, []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
				oldBin, oldFrontend := tmux.Bin, codexHookFrontend
				t.Cleanup(func() { tmux.Bin, codexHookFrontend = oldBin, oldFrontend })
				tmux.Bin = stub
				process, ok := launchhost.ReadAgent(os.Getpid())
				if !ok {
					t.Fatal("could not observe the isolated fixture process")
				}
				process.Comm = "codex"
				t.Setenv(agentProcessEnv, launchhost.EncodeAgent(process))

				r := resume.Record{Agent: "codex", HomeID: homeID, SessionID: root, Cwd: cwd}
				f := resume.CodexFrontend{
					Instance: strings.Repeat("a", 32), HomeID: homeID, SessionID: root, ThreadCwd: cwd,
					BridgePID: process.PID, BridgeStart: process.Start, BackendPID: process.PID, BackendStart: process.Start,
					StartedAt: time.Now().Add(-time.Minute).UnixMilli(), UpdatedAt: time.Now().UnixMilli(), Sequence: 1,
					CodexBindingTarget: resume.CodexBindingTarget{
						Pane: owner, Loc: loc, Cwd: cwd, PID: 101, PanePID: 100,
						ClientStart: "frontend-birth", RootStart: "root-birth", TTY: "test-tty",
					},
				}
				switch phase {
				case "conflicting index":
					r.SessionID = other
				case "expired receipt":
					f.UpdatedAt = time.Now().Add(-10 * time.Second).UnixMilli()
					fallthrough
				case "current binding":
					r.Frontend = f.Instance
					receipt, err := json.Marshal(f)
					if err != nil {
						t.Fatal(err)
					}
					if err := state.WritePrivateFile(filepath.Join(resume.Dir(), "codex-frontends", f.Instance+".json"), receipt); err != nil {
						t.Fatal(err)
					}
				}
				if err := resume.Save(loc, r); err != nil {
					t.Fatal(err)
				}
				if err := resume.Save("dev:0.1", resume.Record{Agent: "codex", HomeID: homeID, SessionID: other, Cwd: cwd}); err != nil {
					t.Fatal(err)
				}
				for pane, sid := range map[string]string{owner: root, peer: other} {
					if err := state.WriteMarker(state.ActivePath(pane), sid); err != nil {
						t.Fatal(err)
					}
					if err := native.Save(native.Record{Agent: "codex", HomeID: homeID, SessionID: sid, State: "working",
						Cwd: cwd, Event: "UserPromptSubmit", UpdatedAt: time.Now().Unix()}); err != nil {
						t.Fatal(err)
					}
				}
				// Mock the strict foreground-client probe, which normally needs
				// a real TTY. The persisted index/receipt, scoped freshness and
				// bridge PID/birth checks still run through actual resume code.
				codexHookFrontend = func(home, session string) (resume.CodexFrontend, bool) {
					active, good := resume.LoadLive(loc)
					return f, good && active.Frontend == f.Instance && active.HomeID == home && active.SessionID == session
				}
				if Run(strings.NewReader(`{"session_id":"root-thread"}`), []string{"--agent", "codex", "--detached", "Stop"}) != 0 {
					t.Fatal("hook failed")
				}
				wantPane, wantNativeState, wantActive := "", "idle", root
				if phase == "current binding" {
					wantPane, wantNativeState, wantActive = owner, "working", ""
				}
				if got := state.ReadMarker(state.ActivePath(owner)); got != wantActive {
					t.Fatalf("owner active = %q, want %q", got, wantActive)
				}
				if got := state.ReadMarker(state.ActivePath(peer)); got != other {
					t.Fatalf("unrelated peer lost its active marker: %q", got)
				}
				if _, ok := state.ReadCompletion(peer); ok {
					t.Fatal("unrelated peer received a completion")
				}
				if completion, ok := state.ReadCompletion(owner); ok != (wantPane != "") || ok && completion.SessionID != root {
					t.Fatalf("wrong completion authority: %+v, present=%t", completion, ok)
				}
				if got, ok := native.Load(root, homeID); !ok || got.State != wantNativeState {
					t.Fatalf("native fallback = %+v, wanted state %q", got, wantNativeState)
				}
				if got, ok := native.Load(other, homeID); !ok || got.State != "working" || got.Event != "UserPromptSubmit" {
					t.Fatalf("unrelated native observation changed: %+v", got)
				}
				records, _ := events.ReadSince(0)
				count := 0
				for _, event := range records {
					if event.Event == "Stop" && event.AgentSession == root {
						count++
						if event.Pane != wantPane {
							t.Fatalf("Stop pane = %q, want %q", event.Pane, wantPane)
						}
					}
				}
				if count != 1 {
					t.Fatalf("expected one Stop event, got %d", count)
				}
			})
		}
	}
}
