package resource

import (
	"encoding/json"
	"strings"
	"testing"
)

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

func TestParseBattery(t *testing.T) {
	ac := parseBattery("Now drawing from 'AC Power'\n -InternalBattery-0 (id=123)\t100%; charged; 0:00 remaining present: true\n")
	if !ac.Present || !ac.OnAC || ac.Percent != 100 || ac.State != "charged" || ac.TimeLeft != "" {
		t.Errorf("AC-charged parse = %+v", ac)
	}
	dis := parseBattery("Now drawing from 'Battery Power'\n -InternalBattery-0 (id=7)\t8%; discharging; 0:23 remaining present: true\n")
	if !dis.Present || dis.OnAC || dis.Percent != 8 || dis.State != "discharging" || dis.TimeLeft != "0:23" {
		t.Errorf("discharging parse = %+v", dis)
	}
	// A desktop (no internal-battery line): present=false, but the AC source is still read.
	desk := parseBattery("Now drawing from 'AC Power'\n")
	if desk.Present || !desk.OnAC {
		t.Errorf("desktop (no battery) = %+v", desk)
	}
}

func TestBatteryTier(t *testing.T) {
	c := cfg()
	onBat := func(pct int) Machine {
		return Machine{Battery: &Battery{Present: true, Percent: pct, State: "discharging"}}
	}
	if batteryTier(onBat(8), c) != TierRed {
		t.Error("8% on battery → red")
	}
	if batteryTier(onBat(15), c) != TierAmber {
		t.Error("15% on battery → amber")
	}
	if batteryTier(onBat(80), c) != TierNormal {
		t.Error("80% on battery → normal")
	}
	// ON AC a low charge is NOT a concern — it's charging, not dying.
	if batteryTier(Machine{Battery: &Battery{Present: true, Percent: 8, OnAC: true, State: "charging"}}, c) != TierNormal {
		t.Error("8% on AC → normal")
	}
	// A desktop / nil battery is always normal.
	if batteryTier(Machine{Battery: &Battery{Present: false, OnAC: true}}, c) != TierNormal {
		t.Error("no internal battery → normal")
	}
	if batteryTier(Machine{}, c) != TierNormal {
		t.Error("nil battery → normal")
	}
}

func TestEvalMachineBattery(t *testing.T) {
	c := cfg()
	// An otherwise-healthy machine on a low DISCHARGING battery: the warn names it and
	// the overall tier reddens (so it rides the existing resource·warn nudge to HQ).
	m := Machine{DiskFreeGB: 200, MemTier: "normal", LoadRatio: 0.3, Battery: &Battery{Present: true, Percent: 8, State: "discharging"}}
	// Every warn NAMES its condition, so a surface can print it as-is instead of colouring
	// a normal-looking reading and hoping the reader infers the rest.
	if w := evalMachine(m, c); w != "battery critical · 8%" {
		t.Errorf("low discharging battery warn = %q, want %q", w, "battery critical · 8%")
	}
	if m.WarnTier(c) != TierRed {
		t.Error("low discharging battery → machine tier red")
	}
	// Same charge but plugged in → not a concern, no warn.
	m.Battery.OnAC = true
	if w := evalMachine(m, c); w != "" {
		t.Errorf("low battery ON AC should not warn: %q", w)
	}
}

// The mobile HQ disc reddens on the exact wire string `"tier":"red"` (and the API
// contract documents `amber`|`red`), so both the json tag and Tier.String()'s output
// are a cross-language contract — renaming either (e.g. "red" → "critical" to match
// MemTier's vocabulary) silently breaks the disc with no other failing test. `role`
// and `sense` have such additive-marshal pins; this gives `tier` the same guard.
func TestTierWireContract(t *testing.T) {
	// Tier.String() wire vocabulary — the values a consumer switches on.
	for _, tc := range []struct {
		tier Tier
		want string
	}{{TierNormal, "normal"}, {TierAmber, "amber"}, {TierRed, "red"}} {
		if got := tc.tier.String(); got != tc.want {
			t.Errorf("Tier(%d).String() = %q, want %q", tc.tier, got, tc.want)
		}
	}
	// A red-tier machine marshals `"tier":"red"` under that exact json key.
	m := Machine{DiskFreeGB: 10, MemTier: "normal"}
	m.Tier = m.WarnTier(cfg()).String()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"tier":"red"`) {
		t.Errorf("red machine must marshal \"tier\":\"red\"; got %s", b)
	}
	// A normal machine omits the field entirely (omitempty) — no stray "tier":"normal".
	healthy := Machine{DiskFreeGB: 500, MemTier: "normal", LoadRatio: 0.2}
	if healthy.WarnTier(cfg()) == TierNormal {
		healthy.Tier = "" // Snapshot leaves it empty when normal
	}
	hb, _ := json.Marshal(healthy)
	if strings.Contains(string(hb), `"tier"`) {
		t.Errorf("healthy machine must omit tier; got %s", hb)
	}
}

// WarnTier is what the mobile HQ disc keys off: only "red" is a genuine bottleneck
// (the disc reddens); a soft "amber" must stay amber so it doesn't masquerade as an
// attention emergency. This pins the 37GB-free case to amber (not red).
func TestWarnTier(t *testing.T) {
	c := cfg()
	cases := []struct {
		name string
		m    Machine
		want Tier
	}{
		{"37GB free (below amber line 50, above red line 15) → amber", Machine{DiskFreeGB: 37, MemTier: "normal"}, TierAmber},
		{"10GB free → red", Machine{DiskFreeGB: 10, MemTier: "normal"}, TierRed},
		{"mem warn only → amber", Machine{DiskFreeGB: 200, MemTier: "warn"}, TierAmber},
		{"mem critical → red", Machine{DiskFreeGB: 200, MemTier: "critical"}, TierRed},
		{"healthy → normal", Machine{DiskFreeGB: 200, MemTier: "normal", LoadRatio: 0.3}, TierNormal},
		{"the worst wins: amber disk + red mem → red", Machine{DiskFreeGB: 37, MemTier: "critical"}, TierRed},
	}
	for _, tc := range cases {
		if got := tc.m.WarnTier(c); got != tc.want {
			t.Errorf("%s: WarnTier = %v, want %v", tc.name, got, tc.want)
		}
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

// A tier is a judgment this package makes, and the warn string is where it says so. The
// disk lines used to read "disk 19GB free" at amber and "disk 19GB free (red)" at red:
// the same sentence for a heads-up and an emergency, and a normal-looking number for
// both. A surface can only colour that and hope.
func TestEveryWarnNamesItsCondition(t *testing.T) {
	c := defaultConfig
	cases := []struct {
		name string
		m    Machine
		want string
	}{
		{"disk amber", Machine{DiskFreeGB: 19}, "disk getting low · 19GB free"},
		{"disk red", Machine{DiskFreeGB: 9}, "disk critical · 9GB free"},
		{"load amber", Machine{DiskFreeGB: 500, LoadRatio: 1.2}, "load high · 1.2×cores"},
		{"load red", Machine{DiskFreeGB: 500, LoadRatio: 2.0}, "load critical · 2.0×cores"},
		{"normal", Machine{DiskFreeGB: 500}, ""},
	}
	for _, tc := range cases {
		if got := evalMachine(tc.m, c); got != tc.want {
			t.Errorf("%s: warn = %q, want %q", tc.name, got, tc.want)
		}
	}
	// The memory lines already worked this way; they are the pattern the others now follow.
	if got := evalMachine(Machine{DiskFreeGB: 500, MemTier: "critical"}, c); got != "memory critical" {
		t.Errorf("memory red = %q", got)
	}
}
