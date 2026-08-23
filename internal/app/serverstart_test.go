package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// isolatedTmux points this test at a throwaway server. NOT t.TempDir(): a unix socket
// path is capped near 104 bytes and the test name in that path overruns it.
func isolatedTmux(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "gtx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "sock")
	if err := os.MkdirAll(sock, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMUX_TMPDIR", sock)
	t.Setenv("TMUX", "")
}

// The launchd path must produce a WORKING server, not merely a submitted job. It is the
// server a reboot's whole working set is restored into, so "the job was accepted" is not
// the thing to assert.
func TestStartBootServerViaLaunchd(t *testing.T) {
	if runtime.GOOS != "darwin" || tmux.Bin == "" {
		t.Skip("darwin + tmux only")
	}
	if _, err := exec.LookPath("launchctl"); err != nil {
		t.Skip("no launchctl")
	}
	isolatedTmux(t)

	if !startBootServerViaLaunchd("gtmux-test-boot") {
		t.Fatal("launchd start reported failure")
	}
	t.Cleanup(func() { _, _ = tmux.Run("kill-server") })

	if got, _ := tmux.Run("list-sessions", "-F", "#{session_name}"); got != "gtmux-test-boot" {
		t.Fatalf("server holds %q, want the boot session", got)
	}

	// Assert what the code actually promises: the environment reached the SERVER, so
	// every pane it later creates inherits it. A launchd-started process gets none of
	// the user's environment — measured here as PATH=/usr/bin:/bin:/usr/sbin:/sbin with
	// no language at all, which means no Homebrew and therefore no tmux, git or agent
	// binaries for anything the server starts.
	//
	// Two assertions were tried before this one and BOTH passed with the environment
	// deliberately stripped, so neither tested anything: a non-ASCII window NAME (those
	// bytes come from the client and are only stored), and non-ASCII a pane PRINTS
	// (tmux 3.7 handles UTF-8 regardless of locale — the v0.11.3 failure was an older
	// tmux). What is still load-bearing today is PATH, and this checks it directly.
	env, _ := tmux.Run("show-environment", "-g")
	if !strings.Contains(env, "PATH="+os.Getenv("PATH")) {
		t.Errorf("the server did not inherit the caller's PATH — it will not find tmux, git or any agent: %q",
			firstLineContaining(env, "PATH="))
	}
}

// The detour is only for a caller that says it is an application; everyone else keeps
// today's path, because their attribution is already honest.
func TestStartBootServerDirectByDefault(t *testing.T) {
	if tmux.Bin == "" {
		t.Skip("no tmux")
	}
	isolatedTmux(t)

	if !startBootServer("gtmux-test-direct", false) {
		t.Fatal("direct start failed")
	}
	t.Cleanup(func() { _, _ = tmux.Run("kill-server") })
	if got, _ := tmux.Run("list-sessions", "-F", "#{session_name}"); got != "gtmux-test-direct" {
		t.Fatalf("server holds %q", got)
	}
}

// Whatever else changes, the working set coming back matters more than the name on a
// permission prompt: an unusable privileged path must fall back, not fail.
func TestStartBootServerFallsBackWhenLaunchdFails(t *testing.T) {
	if tmux.Bin == "" {
		t.Skip("no tmux")
	}
	isolatedTmux(t)
	// An empty PATH makes launchctl unfindable, which is the failure this must survive.
	t.Setenv("PATH", "")

	if !startBootServer("gtmux-test-fallback", true) {
		t.Fatal("a failed launchd start must fall back, not fail the restore")
	}
	t.Cleanup(func() { _, _ = tmux.Run("kill-server") })
	if got, _ := tmux.Run("list-sessions", "-F", "#{session_name}"); got != "gtmux-test-fallback" {
		t.Fatalf("server holds %q", got)
	}
}

// firstLineContaining is a small reporting aid: show the offending line, not a whole
// environment dump, when the assertion above fails.
func firstLineContaining(hay, needle string) string {
	for _, l := range strings.Split(hay, "\n") {
		if strings.Contains(l, needle) {
			return l
		}
	}
	return "(no such line)"
}
