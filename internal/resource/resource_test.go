package resource

import "testing"

func cfg() config { return defaultConfig }

func TestEvalMachine(t *testing.T) {
	c := cfg()
	// 40GB free → amber (default amber line 50GB); mem/load normal
	if w := evalMachine(Machine{DiskFreeGB: 40, MemTier: "normal"}, c); w == "" {
		t.Error("40GB free should warn (amber)")
	}
	// 10GB → red
	if w := evalMachine(Machine{DiskFreeGB: 10, MemTier: "normal"}, c); w == "" || w[:4] != "disk" {
		t.Errorf("10GB should be disk red: %q", w)
	}
	// disk fine but memory critical
	if w := evalMachine(Machine{DiskFreeGB: 200, MemTier: "critical"}, c); w != "memory critical" {
		t.Errorf("mem critical = %q", w)
	}
	// all fine
	if w := evalMachine(Machine{DiskFreeGB: 200, MemTier: "normal", LoadRatio: 0.3}, c); w != "" {
		t.Errorf("healthy machine should not warn: %q", w)
	}
	// load red
	if w := evalMachine(Machine{DiskFreeGB: 200, MemTier: "normal", LoadRatio: 1.6}, c); w == "" {
		t.Error("load 1.6×cores should warn red")
	}
}

// attribute: per-pane RSS/CPU sums the subtree; simulator procs aggregate; an
// agent process outside a pane is NOT flagged; a heavy generic orphan is.
func TestAttribute(t *testing.T) {
	procs := []proc{
		{pid: 100, ppid: 1, rssKB: 50 * 1024, cpu: 1.0, comm: "claude"}, // pane root
		{pid: 101, ppid: 100, rssKB: 30 * 1024, cpu: 2.0, comm: "node"}, // child of pane
		{pid: 200, ppid: 1, rssKB: 20 * 1024, cpu: 0.1, comm: "/Library/Developer/CoreSimulator/x/geod"},
		{pid: 201, ppid: 1, rssKB: 25 * 1024, cpu: 0.2, comm: "/Library/Developer/CoreSimulator/x/testmanagerd"},
		{pid: 300, ppid: 1, rssKB: 400 * 1024, cpu: 5.0, comm: "some-heavy-daemon"}, // generic orphan
		{pid: 400, ppid: 1, rssKB: 500 * 1024, cpu: 3.0, comm: "2.1.207"},           // a stray claude — NOT flagged
	}
	agents, orphans := attribute(procs, map[string]int{"%1": 100}, cfg())
	// pane %1 = 100+101 subtree = 80MB, 3.0% CPU
	if agents["%1"].RSSMB != 80 {
		t.Errorf("pane RSS = %dMB, want 80", agents["%1"].RSSMB)
	}
	var sim, generic, agent bool
	for _, o := range orphans {
		switch {
		case o.Kind == "simulator":
			sim = true
			if o.RSSMB != 45 { // 20+25
				t.Errorf("simulator aggregate RSS = %d, want 45", o.RSSMB)
			}
		case o.PID == 300:
			generic = true
		case o.PID == 400:
			agent = true
		}
	}
	if !sim {
		t.Error("leftover simulator runtime should be one aggregated orphan")
	}
	if !generic {
		t.Error("the heavy generic daemon should be a reclaim candidate")
	}
	if agent {
		t.Error("a stray claude process must NOT be flagged for reclaim")
	}
}

func TestClassifyReclaim(t *testing.T) {
	if k, _ := classifyReclaim("/x/CoreSimulator/y/geod"); k != "simulator" {
		t.Errorf("simulator classify = %q", k)
	}
	if k, _ := classifyReclaim("node /x/vite/bin"); k != "dev-server" {
		t.Errorf("vite classify = %q", k)
	}
	if k, _ := classifyReclaim("random-thing"); k != "" {
		t.Errorf("generic should have empty kind, got %q", k)
	}
}

// A heavy process whose comm is a login-shell marker or junk argv[0] (leading
// '-', e.g. "-PPID"/"-bash") must NOT become a reclaim candidate — regression
// for the "reclaim: -PPID" bug.
func TestReclaimSanityFilter(t *testing.T) {
	for _, c := range []string{"-PPID", "-bash", "-/bin/bash", "", "  "} {
		if validComm(c) {
			t.Errorf("validComm(%q) = true, want false (must not be a reclaim target)", c)
		}
	}
	for _, c := range []string{"node", "/Applications/Foo.app/Contents/MacOS/Foo", "launchd_sim"} {
		if !validComm(c) {
			t.Errorf("validComm(%q) = false, want true", c)
		}
	}
	// end-to-end: a 500 MB "-PPID" orphan candidate is filtered out entirely.
	procs := []proc{{pid: 999, ppid: 1, rssKB: 500 * 1024, cpu: 5, comm: "-PPID"}}
	cfg := defaultConfig
	cfg.OrphanRSSMB = 100 // low, so a 500MB proc WOULD be an orphan if not filtered
	_, orphans := attribute(procs, map[string]int{}, cfg)
	for _, o := range orphans {
		if o.Comm == "-PPID" || o.PID == 999 {
			t.Errorf("junk-comm proc leaked into reclaim: %+v", o)
		}
	}
}
