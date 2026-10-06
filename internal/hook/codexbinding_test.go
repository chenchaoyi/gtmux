package hook

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestCodexSubmitWitnessCorrectsSharedServerPane(t *testing.T) {
	for _, tc := range []struct {
		name, originator string
		pid              string
		tamper, want     bool
		onlyTarget       bool
	}{
		{"same cwd peers", "codex-tui", "58990", false, true, false},
		{"altered prompt", "codex-tui", "58990", true, false, false},
		{"replaced pane process", "codex-tui", "58991", false, false, false},
		{"desktop cannot claim tmux", "Codex Desktop", "58990", false, false, false},
		{"rejected witness cannot use unique cwd", "codex-tui", "58991", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hermeticEnv(t)
			t.Setenv("CODEX_HOME", t.TempDir())
			cwd := t.TempDir()
			now := time.Now().Unix()
			target := resume.CodexBindingTarget{Pane: "%27", Loc: "worker:0.0", Cwd: cwd, PID: 58990, PanePID: 58325}
			wire, err := resume.PrepareCodexBinding(target, "Repair test issue", now)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(os.Getenv("CODEX_HOME"), "sessions", "2026", "10", "03")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			meta, _ := json.Marshal(map[string]any{"type": "session_meta", "payload": map[string]string{"id": "worker-session", "cwd": cwd, "originator": tc.originator}})
			if err := os.WriteFile(filepath.Join(dir, "rollout-2026-10-03T00-00-00-worker-session.jsonl"), append(meta, '\n'), 0o600); err != nil {
				t.Fatal(err)
			}
			stub := filepath.Join(t.TempDir(), "tmux")
			t.Setenv("FAKE_PANE_CWD", cwd)
			t.Setenv("FAKE_CLIENT_PID", tc.pid)
			if tc.onlyTarget {
				t.Setenv("ONLY_TARGET", "1")
			} else {
				t.Setenv("ONLY_TARGET", "")
			}
			script := `#!/bin/sh
case "$*" in
  *list-panes*)
    if [ -z "$ONLY_TARGET" ]; then printf '%s\t%s\t%s\t%s\n' '%18' codex "$FAKE_PANE_CWD" 'dev:0.0'; fi
    printf '%s\t%s\t%s\t%s\n' '%27' codex "$FAKE_PANE_CWD" 'worker:0.0';;
  *pane_pid*) printf '%s\t%s\t%s\t%s\t%s\n' '%27' 'worker:0.0' "$FAKE_PANE_CWD" 58325 codex;;
  *session_name*window_index*) printf '%s\n' 'worker:0.0';;
  *pane_current_path*) printf '%s\n' "$FAKE_PANE_CWD";;
  *session_name*) printf '%s\n' worker;;
esac
`
			if err := os.WriteFile(stub, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			ps := filepath.Join(filepath.Dir(stub), "ps")
			if err := os.WriteFile(ps, []byte("#!/bin/sh\nprintf '%s\\n' '58325 1 bash' \"$FAKE_CLIENT_PID 58325 codex\"\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", filepath.Dir(stub)+string(os.PathListSeparator)+os.Getenv("PATH"))
			// Run each stub once first: macOS assesses a newly written executable on its
			// first run, which under a loaded `go test ./...` can take seconds, and
			// LiveCodexBindingTarget gives ps two. Unwarmed, the Codex-binding tests
			// timed out under a full `make check` on an Intel Mac.
			for _, bin := range []string{stub, ps} {
				_ = exec.Command(bin).Run()
			}
			old := tmux.Bin
			tmux.Bin = stub
			t.Cleanup(func() { tmux.Bin = old })
			t.Setenv("TMUX_PANE", "%18")
			if err := state.WriteMarker(state.ActivePath("%18"), "peer-session"); err != nil {
				t.Fatal(err)
			}
			if tc.tamper {
				wire = "Altered " + wire
			}
			input, _ := json.Marshal(map[string]string{"session_id": "worker-session", "cwd": cwd, "prompt": wire})
			Run(strings.NewReader(string(input)), []string{"--agent", "codex", "--detached", "UserPromptSubmit"})
			rec, bound := resume.Load(target.Loc)
			if bound != tc.want || bound && rec.SessionID != "worker-session" {
				t.Fatalf("hook binding = %+v/%v, want bound=%v", rec, bound, tc.want)
			}
			if got := state.ReadMarker(state.ActivePath("%18")); got != "peer-session" {
				t.Fatal("inherited peer was changed", got)
			}
			if tc.want && state.ReadMarker(state.ActivePath("%27")) != "worker-session" {
				t.Fatal("verified worker did not become active")
			}
		})
	}
}
