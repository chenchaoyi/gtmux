package launchhost

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture() map[int]process {
	return map[int]process{10: {10, 11, "born", "/bin/sh"}, 11: {11, 12, "born", "/opt/bin/codex"}, 12: {12, 13, "born", "/bin/bash"}, 13: {13, 14, "born", "/opt/bin/sample-client"}, 14: {14, 1, "born", "tmux"}}
}
func observeFixture(m map[int]process, app func(string) string) Evidence {
	i := inspector{func(pid int) (process, bool) { p, ok := m[pid]; return p, ok }, app}
	return i.observe(10, []string{"codex"}, 100)
}
func TestHostSnapshotAndAppPreference(t *testing.T) {
	m := fixture()
	e := observeFixture(m, func(string) string { return "" })
	if e.Name != "sample-client" || e.Kind != "process" || e.AgentPID != 11 || e.HostPID != 13 || e.ObservedAt != 100 {
		t.Fatal(e)
	}
	e = observeFixture(m, func(string) string { return "Sample Workbench" })
	if e.Kind != "app" || e.Name != "Sample Workbench" {
		t.Fatal(e)
	}
	if Decode(Encode(e), 100) != e {
		t.Fatal("handoff changed evidence")
	}
}
func TestAmbiguousOrMissingAncestryStaysUnknown(t *testing.T) {
	for _, name := range []string{"node", "python3.14", "bun", "java", "tmux", "launchd", "systemd", "cmd --secret", "1.2.3", "bad\x1bname"} {
		t.Run(name, func(t *testing.T) {
			m := fixture()
			p := m[13]
			p.Command = name
			m[13] = p
			if e := observeFixture(m, func(string) string { return "" }); e != (Evidence{}) {
				t.Fatal(e)
			}
		})
	}
	for _, kind := range []string{"wrong agent", "cycle", "missing", "depth"} {
		t.Run(kind, func(t *testing.T) {
			m := fixture()
			switch kind {
			case "wrong agent":
				p := m[11]
				p.Command = "unknown"
				m[11] = p
			case "cycle":
				p := m[12]
				p.Parent = 10
				m[12] = p
			case "missing":
				delete(m, 12)
			case "depth":
				for pid := 12; pid < 25; pid++ {
					m[pid] = process{pid, pid + 1, "born", "sh"}
				}
			}
			if e := observeFixture(m, func(string) string { return "" }); e != (Evidence{}) {
				t.Fatal(e)
			}
		})
	}
}
func TestChangedProcessIdentityRejectsWholeChain(t *testing.T) {
	for _, kind := range []string{"birth", "parent", "command"} {
		t.Run(kind, func(t *testing.T) {
			m := fixture()
			reads := map[int]int{}
			i := inspector{read: func(pid int) (process, bool) {
				p, ok := m[pid]
				reads[pid]++
				if pid == 11 && reads[pid] > 1 {
					switch kind {
					case "birth":
						p.Start = "new"
					case "parent":
						p.Parent = 99
					case "command":
						p.Command = "other"
					}
				}
				return p, ok
			}, app: func(string) string { return "" }}
			if e := i.observe(10, []string{"codex"}, 100); e != (Evidence{}) {
				t.Fatal(e)
			}
		})
	}
}
func TestHandoffRejectsStaleFutureAndOversizedEvidence(t *testing.T) {
	e := observeFixture(fixture(), func(string) string { return "" })
	raw := Encode(e)
	for _, r := range []string{"", strings.Repeat("x", 1025), strings.Replace(raw, "sample-client", "bad/name", 1)} {
		if Decode(r, 100) != (Evidence{}) {
			t.Fatal(r)
		}
	}
	for _, now := range []int64{161, 97} {
		if Decode(raw, now) != (Evidence{}) {
			t.Fatal(now)
		}
	}
}
func fakePS(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ps")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	return dir
}
func TestProcessReadOnlyRequestsNarrowMetadata(t *testing.T) {
	dir := fakePS(t, `printf '%s\n' "$@" > "$ARGFILE"
printf '%s\n' '2026 Mon Oct 19 10:11:12 2026 /Applications/Sample App.app/Contents/MacOS/Sample App'
`)
	args := filepath.Join(dir, "args")
	t.Setenv("ARGFILE", args)
	p, ok := readProcess(context.Background(), 9)
	if !ok || p.Parent != 2026 || p.Command != "/Applications/Sample App.app/Contents/MacOS/Sample App" {
		t.Fatal(p, ok)
	}
	b, err := os.ReadFile(args)
	if err != nil || string(b) != "-p\n9\n-o\nppid=,lstart=,comm=\n" {
		t.Fatal(string(b), err)
	}
}
func TestProcessBudgetAndOutputCap(t *testing.T) {
	fakePS(t, "exec sleep 5\n")
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	start := time.Now()
	if _, ok := readProcess(ctx, 9); ok || time.Since(start) > 2*time.Second {
		t.Fatal("deadline not enforced")
	}
	fakePS(t, "printf '%06000d' 0\n")
	if _, ok := readProcess(context.Background(), 9); ok {
		t.Fatal("oversized output accepted")
	}
}
