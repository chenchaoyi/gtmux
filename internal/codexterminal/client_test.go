package codexterminal

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

const fixture = "100 1 100 200 ttys001 Mon Oct 19 00:00:00 2026 /bin/bash\n" +
	"200 100 200 200 ttys001 Mon Oct 19 00:00:01 2026 /opt/sample-launcher\n" +
	"300 200 200 200 ttys001 Mon Oct 19 00:00:02 2026 /opt/codex\n" +
	"400 300 200 200 ttys001 Mon Oct 19 00:00:03 2026 /opt/codex\n"

func TestWrappedCodexRequiresForegroundTerminalInput(t *testing.T) {
	for _, kind := range []string{"verified", "other harness", "version-named harness", "pipe", "background", "detached", "other tty", "missing root", "client reused", "root reused", "reparented", "pane changed", "stdin changed", "ambiguous", "new ambiguous branch", "unrelated client", "cycle"} {
		t.Run(kind, func(t *testing.T) {
			before, after := parseProcesses(fixture), parseProcesses(fixture)
			pane := Pane{ID: "%7", Command: "sample-launcher", PID: 100, TTY: "/dev/ttys001"}
			mutate := func(m map[int]process, pid int, fn func(*process)) { p := m[pid]; fn(&p); m[pid] = p }
			switch kind {
			case "other harness":
				pane.Command = "claude"
			case "version-named harness":
				pane.Command = "2.1.250"
			case "background":
				mutate(before, 300, func(p *process) { p.group = 900 })
			case "detached":
				mutate(before, 300, func(p *process) { p.tty = ""; p.foreground = -1 })
			case "other tty":
				mutate(before, 300, func(p *process) { p.tty = "ttys002" })
			case "missing root":
				delete(before, 100)
			case "client reused":
				mutate(after, 300, func(p *process) { p.start = "new birth" })
			case "root reused":
				mutate(after, 100, func(p *process) { p.start = "new birth" })
			case "reparented":
				mutate(after, 200, func(p *process) { p.parent = 1 })
			case "ambiguous", "new ambiguous branch":
				p := before[300]
				p.pid = 301
				after[301] = p
				if kind == "ambiguous" {
					before[301] = p
				}
			case "unrelated client":
				p := before[300]
				p.pid = 301
				p.parent = 1
				before[301] = p
				after[301] = p
			case "cycle":
				mutate(before, 100, func(p *process) { p.parent = 200 })
			}
			if kind == "background" || kind == "detached" || kind == "other tty" {
				after[300] = before[300]
			}
			snapshots, inputs, panes := 0, 0, 0
			i := inspector{
				snapshot: func() map[int]process {
					snapshots++
					if snapshots == 1 {
						return before
					}
					return after
				},
				stdin: func(int) string {
					inputs++
					if kind == "pipe" || kind == "stdin changed" && inputs > 1 {
						return ""
					}
					return "/dev/ttys001"
				},
				pane: func(Pane) bool { panes++; return kind != "pane changed" },
			}
			got := i.resolve([]Pane{pane})
			want := kind == "verified" || kind == "unrelated client"
			c, accepted := got[pane.ID]
			if accepted != want {
				t.Fatalf("qualification=%+v want accepted=%v", got, want)
			}
			if want {
				if c.PID != 300 || c.Start == "" || c.RootStart == "" || c.TTY != "ttys001" || snapshots != 2 || inputs != 2 || panes != 1 {
					t.Fatalf("missing revalidation: %+v snapshots=%d stdin=%d pane=%d", c, snapshots, inputs, panes)
				}
			}
		})
	}
}

func TestWrappedCodexBoundsAndProcessParsing(t *testing.T) {
	p := parseProcesses(fixture)
	if len(p) != 4 || p[300].parent != 200 || p[300].command != "/opt/codex" {
		t.Fatalf("parsed snapshot: %+v", p)
	}
	if parseProcesses(fixture+fixture) != nil {
		t.Fatal("duplicate process identities accepted")
	}
	if len(parseProcesses("300 200 200 200 tty fake date /opt/codex\n")) != 0 {
		t.Fatal("invalid process birth accepted")
	}
	for _, s := range []string{"", "?", "??", "-", "pipe", "/tmp/ttys001", "socket:[1]"} {
		if tty(s) != "" {
			t.Errorf("non-terminal %q accepted", s)
		}
	}
	if tty("/dev/pts/1") != "pts/1" {
		t.Fatal("Linux pseudo-terminal not normalized")
	}
	var w limitedOutput
	data := []byte(strings.Repeat("x", (2<<20)+1))
	n, err := w.Write(data)
	if err != nil || n != len(data) || !w.large || len(w.data) != 0 {
		t.Fatal("oversized subprocess output not discarded")
	}
	// More than the traversal budget must never produce a partial positive result.
	for n := 500; n < 800; n++ {
		p[n] = process{pid: n, parent: 100, start: "birth", command: "helper"}
	}
	if c, _ := findClient(Pane{ID: "%7", PID: 100, TTY: "ttys001"}, p); c.pid != 0 {
		t.Fatal("over-budget traversal produced an owner")
	}
}

func TestWrappedCodexCommandDeadline(t *testing.T) {
	t.Setenv("GTMUX_CODEX_TERMINAL_SLOW_CHILD", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	at := time.Now()
	if run(ctx, os.Args[0], "-test.run=^TestCodexTerminalSlowChild$") != "" {
		t.Fatal("timed-out command returned evidence")
	}
	if time.Since(at) > 500*time.Millisecond {
		t.Fatal("shared deadline did not bound the command")
	}
}
func TestCodexTerminalSlowChild(t *testing.T) {
	if os.Getenv("GTMUX_CODEX_TERMINAL_SLOW_CHILD") == "1" {
		time.Sleep(time.Second)
	}
}

func TestWrappedCodexPaneRevalidation(t *testing.T) {
	p := Pane{ID: "%7", PID: 100, TTY: "/dev/ttys001", Command: "sample-launcher", Cwd: "/example", Loc: "work:0.0"}
	fields := []string{"%7", "100", "/dev/ttys001", "sample-launcher", "/example", "work:0.0"}
	if !paneMatches(p, strings.Join(fields, "\t")) {
		t.Fatal("unchanged pane rejected")
	}
	for index := range fields {
		changed := append([]string(nil), fields...)
		changed[index] = "changed"
		if paneMatches(p, strings.Join(changed, "\t")) {
			t.Fatalf("changed pane field%d authorized a stale candidate", index)
		}
	}
	if paneMatches(p, "%7\t100") {
		t.Fatal("incomplete pane accepted")
	}
}
