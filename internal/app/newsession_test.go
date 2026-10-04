package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestRemoteSessionCreationOnIsolatedTmux(t *testing.T) {
	bin, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux unavailable")
	}
	dir, err := os.MkdirTemp("/tmp", "gtx-new-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMUX", "")
	sock := filepath.Join(dir, "tmux.sock")
	wrapper := filepath.Join(dir, "tmux-wrapper")
	body := "#!/bin/sh\nexec " + shellQuote(bin) + " -S " + shellQuote(sock) + " -f /dev/null \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	old := tmux.Bin
	tmux.Bin = wrapper
	t.Cleanup(func() { _ = tmux.OK("kill-server"); tmux.Bin = old })
	request := "remote-request-123456"
	first, err := createDetachedSession(" Project.v1:Review ", request, dir)
	if err != nil {
		t.Fatal(err)
	}
	if first.Session != "Project-v1-Review" || first.Loc != first.Session+":"+first.Window+"."+first.Pane {
		t.Fatalf("bad receipt: %+v", first)
	}
	if got := tmux.Display(first.PaneID, "#{pane_current_path}"); got != dir {
		t.Fatalf("cwd %q, want %q", got, dir)
	}
	if got := tmux.Display(first.PaneID, "#{session_attached}"); got != "0" {
		t.Fatalf("created session is not detached: %s", got)
	}
	if !tmux.OK("split-window", "-d", "-t", first.PaneID) {
		t.Fatal("split failed")
	}
	again, err := createDetachedSession("Project.v1:Review", request, dir)
	if err != nil || again.Session != first.Session {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if _, err := createDetachedSession("Changed", request, dir); !errors.Is(err, errSessionRequestChanged) {
		t.Fatalf("changed request %v", err)
	}
	if _, err := createDetachedSession(first.Session, "another-request-1234", dir); !errors.Is(err, errSessionNameExists) {
		t.Fatalf("duplicate %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := createDetachedSession("", "automatic-request-1234", dir)
			if e != nil {
				errs <- e
			} else if r.Session == "" {
				errs <- errors.New("empty automatic name")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	sessions := tmux.Lines("list-sessions", "-F", "#{session_name}")
	if len(sessions) != 2 {
		t.Fatalf("repeated requests made %d sessions: %v", len(sessions), sessions)
	}
	// A new process (like serve restarting) has no in-memory request cache.
	code := `#!/bin/sh
exec ` + shellQuote(bin) + ` -S ` + shellQuote(sock) + ` list-panes -a -F '#{GTMUX_CREATE_ID}'
`
	probe := filepath.Join(dir, "probe")
	_ = os.WriteFile(probe, []byte(code), 0700)
	got, err := exec.Command(probe).Output()
	if err != nil || !strings.Contains(string(got), request) {
		t.Fatalf("receipt missing across process: %s %v", got, err)
	}
}

func TestSessionNameValidation(t *testing.T) {
	for _, name := range []string{"中文 项目", "", "literal $(echo nope); 'quote'"} {
		if !validSessionName(name) {
			t.Errorf("valid name rejected %q", name)
		}
	}
	for _, name := range []string{"a\nb", "a\x00b", strings.Repeat("a", 257)} {
		if validSessionName(name) {
			t.Errorf("invalid name accepted %q", name)
		}
	}
}

func TestSessionReceiptLookupFailureDoesNotCreate(t *testing.T) {
	wrapper := filepath.Join(t.TempDir(), "tmux-wrapper")
	body := "#!/bin/sh\nif [ \"$2\" = list-panes ]; then echo 'permission denied' >&2; exit 1; fi\necho unexpected creation >&2; exit 0\n"
	if err := os.WriteFile(wrapper, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	previous := tmux.Bin
	tmux.Bin = wrapper
	t.Cleanup(func() { tmux.Bin = previous })
	_, err := createDetachedSession("work", "lookup-failure-request", "")
	if err == nil || !strings.Contains(err.Error(), "could not check session receipt") {
		t.Fatalf("lookup failure hidden: %v", err)
	}
}

// A session gtmux creates without being told a directory never starts at the root of the
// disk: the menu-bar app and serve run in "/", and "New session" from either opened a
// shell there (2026-10-05). A real working directory is kept.
func TestSessionStartDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir("/"); err != nil {
		t.Fatal(err)
	}
	if got := sessionStartDir(); got != home {
		t.Fatalf("from /: %q, want home %q", got, home)
	}
	project := t.TempDir()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	if got := sessionStartDir(); got != "" {
		t.Fatalf("from a project: %q, want \"\" (tmux keeps the working directory)", got)
	}
}

// End to end on an isolated tmux server: created from "/", the new pane's shell is in home.
func TestANewSessionFromTheRootStartsAtHome(t *testing.T) {
	bin, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux unavailable")
	}
	dir, err := os.MkdirTemp("/tmp", "gtx-home-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	dir, _ = filepath.EvalSymlinks(dir)
	home := filepath.Join(dir, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("TMUX", "")
	sock := filepath.Join(dir, "tmux.sock")
	wrapper := filepath.Join(dir, "tmux-wrapper")
	body := "#!/bin/sh\nexec " + shellQuote(bin) + " -S " + shellQuote(sock) + " -f /dev/null \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	old := tmux.Bin
	tmux.Bin = wrapper
	t.Cleanup(func() { _ = tmux.OK("kill-server"); tmux.Bin = old })
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir("/"); err != nil { // where the menu-bar app and serve run
		t.Fatal(err)
	}
	s, err := createDetachedSession("from-root", "", sessionStartDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := tmux.Display(s.PaneID, "#{pane_current_path}"); got != home {
		t.Fatalf("the new session started in %q, want home %q", got, home)
	}
}
