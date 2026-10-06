package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestAdoptSessionName(t *testing.T) {
	cases := map[string]string{
		"/Users/x/proj/acme-mobile": "acme-mobile",
		"/Users/x/my.proj":          "my-proj", // '.' → '-'
		"/Users/x/a b":              "a-b",     // space → '-'
		"/tmp/":                     "tmp",     // trailing slash
		"/":                         "",        // nothing usable
		"":                          "",
	}
	for cwd, want := range cases {
		if got := adoptSessionName(cwd); got != want {
			t.Errorf("adoptSessionName(%q) = %q, want %q", cwd, got, want)
		}
	}
}

func TestAdoptRejectsChatGPTDesktopBeforeSpawning(t *testing.T) {
	for _, originator := range []string{"codex_work_desktop", "Codex Desktop"} {
		t.Run(originator, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			codexHome := t.TempDir()
			t.Setenv("CODEX_HOME", codexHome)
			dir := filepath.Join(codexHome, "sessions", "2026", "09", "29")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			const id = "desktop-to-keep"
			data := `{"type":"session_meta","payload":{"id":"` + id + `","originator":"` + originator + `"}}` + "\n"
			if err := os.WriteFile(filepath.Join(dir, "rollout-2026-09-29T00-00-00-"+id+".jsonl"), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := native.Save(native.Record{SessionID: id, Agent: "codex", State: "idle", UpdatedAt: time.Now().Unix()}); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(t.TempDir(), "spawned")
			fakeTmux := filepath.Join(t.TempDir(), "tmux")
			if err := os.WriteFile(fakeTmux, []byte("#!/bin/sh\ntouch '"+marker+"'\nexit 1\n"), 0o700); err != nil {
				t.Fatal(err)
			}
			saved := tmux.Bin
			tmux.Bin = fakeTmux
			t.Cleanup(func() { tmux.Bin = saved })
			if got := cmdAdopt([]string{id}); got != 1 {
				t.Fatalf("desktop adopt exit = %d, want refusal", got)
			}
			if _, ok := native.Load(id); !ok {
				t.Fatal("refused desktop session was removed")
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("desktop refusal attempted to spawn tmux")
			}
		})
	}
}

// adoptRig is a native Codex conversation on disk, its "original" process (a sleep this
// test owns), and a stand-in tmux that logs every call and fails where a case asks.
type adoptRig struct {
	t        *testing.T
	id       string
	orig     *exec.Cmd
	log      string
	paneFile string
	sendFail string
	killFail string
}

func newAdoptRig(t *testing.T, state string, onDisk bool) *adoptRig {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	r := &adoptRig{t: t, id: "move-me"}
	if onDisk {
		dir := filepath.Join(codexHome, "sessions", "2026", "10", "06")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		data := `{"timestamp":"2026-10-06T10:00:00Z","type":"session_meta","payload":{"id":"` + r.id + `","originator":"codex_cli_rs","cwd":"/repo"}}` + "\n" +
			`{"timestamp":"2026-10-06T10:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"fix it"}]}}` + "\n" +
			`{"timestamp":"2026-10-06T10:00:03Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Fixed."}]}}` + "\n"
		if err := os.WriteFile(filepath.Join(dir, "rollout-2026-10-06T10-00-00-"+r.id+".jsonl"), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r.orig = exec.Command("sleep", "60")
	if err := r.orig.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.orig.Process.Kill(); _ = r.orig.Wait() })
	if err := native.Save(native.Record{SessionID: r.id, Agent: "codex", State: state, Cwd: "/repo",
		PID: r.orig.Process.Pid, Comm: "sleep", UpdatedAt: time.Now().Unix()}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	r.log, r.paneFile, r.sendFail, r.killFail = filepath.Join(dir, "calls"), filepath.Join(dir, "pane"), filepath.Join(dir, "send-fails"), filepath.Join(dir, "kill-fails")
	if err := os.WriteFile(r.paneFile, []byte("%91"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(dir, "tmux")
	script := "#!/bin/sh\necho \"$*\" >> '" + r.log + "'\n[ \"$1\" = -u ] && shift\ncase \"$1\" in\n" +
		"new-session) echo adopted ;;\n" +
		"kill-session) [ -e '" + r.killFail + "' ] && exit 9 ;;\n" +
		"display-message) case \"$*\" in *pane_id*) cat '" + r.paneFile + "' ;; *pane_current_command*) echo zsh ;; esac ;;\n" +
		"send-keys) [ -e '" + r.sendFail + "' ] && exit 1 ;;\n" +
		"esac\nexit 0\n"
	if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	saved := tmux.Bin
	tmux.Bin = fake
	t.Cleanup(func() { tmux.Bin = saved })
	// No case opens a terminal tab: only a successful move gets that far, and that case
	// replaces this with its own stub. Reaching it anywhere else is the bug under test.
	savedTabs := adoptOpenTabs
	adoptOpenTabs = func([]string) (bool, string, error) {
		t.Error("adopt tried to open a terminal tab")
		return false, "", nil
	}
	t.Cleanup(func() { adoptOpenTabs = savedTabs })
	return r
}

func (r *adoptRig) calls() string {
	b, _ := os.ReadFile(r.log)
	return string(b)
}

func (r *adoptRig) origAlive() bool {
	return r.orig.Process.Signal(syscall.Signal(0)) == nil && r.orig.ProcessState == nil
}

func (r *adoptRig) ready(ok bool) {
	saved := adoptWaitReady
	adoptWaitReady = func(string, string) bool { return ok }
	r.t.Cleanup(func() { adoptWaitReady = saved })
}

// `gtmux adopt` asks the radar's own question before creating anything: a session that
// is mid-turn, or has nothing on disk, is refused without a tmux call (%12, 2026-10-06:
// the command checked only the desktop client and resumability).
func TestAdoptRefusesWhatTheRadarWouldNotOffer(t *testing.T) {
	for _, c := range []struct {
		name, state string
		onDisk      bool
	}{{"working", "working", true}, {"waiting", "waiting", true}, {"nothing on disk", "idle", false}} {
		t.Run(c.name, func(t *testing.T) {
			r := newAdoptRig(t, c.state, c.onDisk)
			r.ready(true)
			if got := cmdAdopt([]string{r.id}); got != 1 {
				t.Fatalf("exit = %d, want a refusal", got)
			}
			if calls := r.calls(); calls != "" {
				t.Errorf("tmux was called for a refused session:\n%s", calls)
			}
			if _, ok := native.Load(r.id); !ok || !r.origAlive() {
				t.Error("a refused session lost its record or its process")
			}
		})
	}
}

// A failure after the new session exists removes that session and leaves the original
// running and listed. It used to close the original and drop its record anyway, so a
// pane that never got the command cost the user their live conversation.
func TestAdoptFailureKeepsTheOriginal(t *testing.T) {
	for _, c := range []struct {
		name  string
		setup func(*adoptRig)
	}{
		{"the new session has no pane", func(r *adoptRig) { _ = os.WriteFile(r.paneFile, nil, 0o600); r.ready(true) }},
		{"the resume command is not typed", func(r *adoptRig) { _ = os.WriteFile(r.sendFail, nil, 0o600); r.ready(true) }},
		{"the resumed agent never comes up", func(r *adoptRig) { r.ready(false) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newAdoptRig(t, "idle", true)
			c.setup(r)
			if got := cmdAdopt([]string{r.id}); got != 1 {
				t.Fatalf("exit = %d, want failure", got)
			}
			if !strings.Contains(r.calls(), "kill-session -t adopted") {
				t.Errorf("the session adopt created was left behind:\n%s", r.calls())
			}
			time.Sleep(50 * time.Millisecond)
			if !r.origAlive() {
				t.Error("the original process was closed although the move failed")
			}
			if _, ok := native.Load(r.id); !ok {
				t.Error("the original's record was dropped although the move failed")
			}
		})
	}
}

// Only once the resumed agent has taken the new pane over is the original closed and its
// record dropped.
func TestAdoptClosesTheOriginalOnlyAfterTheResumeIsUp(t *testing.T) {
	r := newAdoptRig(t, "idle", true)
	r.ready(true)
	saved := adoptOpenTabs
	adoptOpenTabs = func([]string) (bool, string, error) { return false, "", nil } // never open a real tab
	t.Cleanup(func() { adoptOpenTabs = saved })
	if got := cmdAdopt([]string{r.id}); got != 0 {
		t.Fatalf("exit = %d, want success; tmux calls:\n%s", got, r.calls())
	}
	if strings.Contains(r.calls(), "kill-session") {
		t.Errorf("a successful move removed its own session:\n%s", r.calls())
	}
	_ = r.orig.Wait() // SIGTERM ends the sleep
	if _, ok := native.Load(r.id); ok {
		t.Error("the moved conversation is still listed outside tmux")
	}
}

// When the session adopt created cannot be removed, the failure says which one is left,
// so a retry does not start a second resumed agent beside it; the original is still kept
// (%12's review, 2026-10-06: the kill-session error was dropped).
func TestAdoptSaysWhichSessionItCouldNotRemove(t *testing.T) {
	r := newAdoptRig(t, "idle", true)
	r.ready(false)
	_ = os.WriteFile(r.killFail, nil, 0o600)
	stderr := captureStderr(t, func() {
		if got := cmdAdopt([]string{r.id}); got != 1 {
			t.Fatalf("exit = %d, want failure", got)
		}
	})
	if !strings.Contains(stderr, "tmux kill-session -t adopted") || !strings.Contains(stderr, "could not be removed") {
		t.Errorf("the failure does not name the session left behind:\n%s", stderr)
	}
	if strings.Contains(stderr, "was removed") {
		t.Errorf("the failure claims a cleanup that did not happen:\n%s", stderr)
	}
	time.Sleep(50 * time.Millisecond)
	if _, ok := native.Load(r.id); !ok || !r.origAlive() {
		t.Error("the original lost its record or its process")
	}
}
