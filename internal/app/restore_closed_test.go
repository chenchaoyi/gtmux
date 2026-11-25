package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestClosedRestoreHookChild(t *testing.T) {
	if os.Getenv("GTMUX_CLOSED_RESTORE_HOOK_TEST") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "restore" {
			os.Exit(Run(os.Args[i:]))
		}
	}
	os.Exit(2)
}

func TestClosedRestoreUsesPrivateTmux(t *testing.T) {
	if tmux.Bin == "" {
		t.Skip("tmux unavailable")
	}
	root, err := os.MkdirTemp("/tmp", "gcr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	t.Setenv("HOME", root)
	t.Setenv("TMUX_TMPDIR", root)
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("GTMUX_CLOSED_RESTORE_HOOK_TEST", "1")
	run := func(args ...string) string {
		t.Helper()
		out, err := tmux.Run(args...)
		if err != nil {
			t.Fatalf("private tmux %v: %v", args, err)
		}
		return out
	}
	run("-f", "/dev/null", "new-session", "-d", "-s", "anchor", "sleep 40")
	t.Cleanup(func() { _ = tmux.OK("kill-server") })
	run("set-hook", "-g", "session-closed[0]", "display-message 'user hook kept'")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(root, "hook")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexec "+shellQuote(exe)+" -test.run=^TestClosedRestoreHookChild$ -- \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	installRestoreClosedHook(hook)
	installRestoreClosedHook(hook)
	hooks := run("show-hooks", "-g", "session-closed")
	if !strings.Contains(hooks, "user hook kept") || strings.Count(hooks, "--record-closed") != 1 {
		t.Fatalf("unexpected hook array: %s", hooks)
	}

	closed := []string{"closed one", "关闭 'two'"}
	panes := make([]string, 0, len(closed))
	for _, name := range closed {
		panes = append(panes, run("new-session", "-d", "-P", "-F", "#{pane_id}", "-s", name, "/bin/sh"))
	}
	directory := filepath.Join(root, ".local", "share", "tmux", "resurrect")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	save := filepath.Join(directory, "last")
	archive := func(names ...string) string {
		var b strings.Builder
		for _, name := range names {
			b.WriteString("pane\t" + name + "\t0\t1\t:*\t0\tsh\t:" + root + "\t1\tbash\t:\n")
			b.WriteString("window\t" + name + "\t0\t:sh\t1\t:*\tlayout\n")
		}
		return b.String()
	}
	original := archive("anchor", closed[0], closed[1], "missing")
	if err := os.WriteFile(save, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	// Tie the fixture to this server's lifetime without sleeping across the
	// second-resolution tmux start_time boundary.
	mtime := time.Now().Add(3 * time.Second)
	if err := os.Chtimes(save, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	preview := func() []string {
		t.Helper()
		p, err := pendingRestorePlan(save, tmux.Run)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, row := range p.Sessions {
			names = append(names, row.Name+":"+row.State)
		}
		slices.Sort(names)
		return names
	}
	for _, pane := range panes {
		run("send-keys", "-t", pane, "exit", "Enter")
	}
	for end := time.Now().Add(4 * time.Second); ; {
		view := preview()
		if slices.Equal(view, []string{"anchor:detached", "missing:missing"}) {
			break
		}
		if time.Now().After(end) {
			t.Fatalf("closures still suggested: %v; evidence names=%v", view, readRestoreClosed().Names)
		}
		time.Sleep(30 * time.Millisecond)
	}
	remote, err := remoteRestorePlanWith(save, tmux.Run, time.Now())
	if err != nil || len(remote.Sessions) != 1 || remote.Sessions[0].Name != "missing" {
		t.Fatalf("remote plan includes closed session: %+v (%v)", remote.Sessions, err)
	}

	// Exercise the real restore subprocess with a consumer using the
	// tmux-resurrect archive lookup. Its tmux mutators still go to this socket.
	plugin := filepath.Join(root, ".tmux", "plugins", "tmux-resurrect", "scripts")
	if err := os.MkdirAll(plugin, 0700); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(plugin, "restore.sh")
	if err := os.WriteFile(filepath.Join(plugin, "helpers.sh"), []byte("option=@resurrect-dir\ntmux show-option -gqv \"$option\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	sent := filepath.Join(root, "consumer")
	body := `#!/bin/sh
set -eu
[ "$(tmux show-option -gqv @resurrect-never-overwrite)" = on ]
save=$(tmux show-option -gqv @resurrect-dir)/last
cp "$save" ` + shellQuote(sent) + `
if grep -q "$(printf 'pane\tmissing\t')" "$save"; then
    tmux -f /dev/null new-session -d -s missing 'sleep 35'
fi
`
	if err := os.WriteFile(script, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	originalResumeFlag := restoreResumeFlag
	restoreResumeFlag = "off"
	t.Cleanup(func() { restoreResumeFlag = originalResumeFlag })
	recoverMissingSavedSessions()
	if got, err := os.ReadFile(sent); err != nil || string(got) != archive("anchor", "missing") {
		t.Fatalf("consumer archive contains wrong sessions: %v", err)
	}
	if got, _ := os.ReadFile(save); string(got) != original || run("show-option", "-gqv", "@resurrect-dir") != "" ||
		run("show-option", "-gqv", "@resurrect-never-overwrite") != "" {
		t.Fatal("original archive or configured Resurrect settings changed")
	}

	run("new-session", "-d", "-s", closed[0], "sleep 35")
	if got := preview(); !slices.Contains(got, closed[0]+":detached") {
		t.Fatalf("new session of same name hidden: %v", got)
	}
	if err := os.WriteFile(save, []byte(original+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := preview(); !slices.Contains(got, closed[1]+":missing") {
		t.Fatalf("closure incorrectly applied to different snapshot: %v", got)
	}
	if err := os.WriteFile(save, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(save, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	layout := loadSavedLayout(save)
	staged, err := stageClosedRestore(script, layout, restoreClosedForLayout(layout, tmux.Run))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(staged) })
	tryStaleStage := func() {
		t.Helper()
		cmd := exec.Command(filepath.Join(staged, "tmux"), "new-session", "-d", "-s", "stale-stage", "sleep 30")
		if err := cmd.Run(); err == nil {
			t.Fatal("a staged restore changed a different tmux server")
		}
	}
	run("kill-server")
	tryStaleStage()
	if tmux.ServerUp() {
		t.Fatal("a staged restore started a replacement server")
	}
	if got := preview(); !slices.Contains(got, closed[1]+":missing") {
		t.Fatalf("closure incorrectly applied to failed server: %v", got)
	}
	run("-f", "/dev/null", "new-session", "-d", "-s", "new-anchor", "sleep 30")
	tryStaleStage()
	if got := run("list-sessions", "-F", "#{session_name}"); got != "new-anchor" {
		t.Fatalf("staged restore changed the replacement server: %q", got)
	}
	if got := preview(); !slices.Contains(got, closed[1]+":missing") {
		t.Fatalf("closure incorrectly applied to restarted server: %v", got)
	}
}
