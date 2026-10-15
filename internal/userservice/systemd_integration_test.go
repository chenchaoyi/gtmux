package userservice

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Enabled only in an ephemeral Linux CI account with its own real user manager.
// Never run against a developer's installed gtmux units or tmux sessions.
func TestSystemdUserManagerLifecycle(t *testing.T) {
	if os.Getenv("GTMUX_SYSTEMD_IT") != "1" {
		t.Skip("requires isolated CI systemd account")
	}
	home := os.Getenv("HOME")
	m := Systemd{Home: home, Run: Command, SearchPath: "/usr/bin:/bin"}
	if err := m.Check(); err != nil {
		t.Fatal(err)
	}
	label := "com.gtmux.ci-test-" + strconv.Itoa(os.Getpid())
	path := m.Path(label)
	pidfile := filepath.Join(home, "child.pid")
	var children []int
	t.Cleanup(func() {
		_ = m.Stop(path)
		for _, pid := range children {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
		_ = os.Remove(path)
	})
	// A detached tmux created by serve belongs to its cgroup but must outlive a
	// serve restart. A synthetic sleeping child tests that lifecycle invariant.
	script := "echo service-start; sleep 300 & echo $! > '" + pidfile + "'; wait"
	if err := m.Write(path, label, []string{"/bin/sh", "-c", script}, filepath.Join(home, "service output.log")); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(path); err != nil {
		t.Fatal(err)
	}
	readChild := func(previous int) int {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			b, _ := os.ReadFile(pidfile)
			pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
			if pid > 0 && pid != previous {
				children = append(children, pid)
				return pid
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("service did not run literal shell arguments / write its child receipt")
		return 0
	}
	first := readChild(0)
	if log, err := os.ReadFile(filepath.Join(home, "service output.log")); err != nil || !strings.Contains(string(log), "service-start") {
		t.Fatalf("service capture missing: %s %v", log, err)
	}
	if !m.Active(label) {
		t.Fatal("started unit is inactive")
	}
	if err := m.Restart(label); err != nil {
		t.Fatal(err)
	}
	_ = readChild(first)
	if !processAlive(first) {
		t.Fatal("service restart killed existing agent work")
	}
	if err := m.Stop(path); err != nil {
		t.Fatal(err)
	}
	if m.Active(label) {
		t.Fatal("stopped unit is still active")
	}
	if !processAlive(first) {
		t.Fatal("disabling remote access killed detached work")
	}
}

func processAlive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	_, tail, ok := strings.Cut(string(b), ") ")
	return ok && len(tail) > 0 && tail[0] != 'Z' && tail[0] != 'X'
}
