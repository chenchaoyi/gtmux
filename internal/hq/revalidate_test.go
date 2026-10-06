package hq

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/resource"
)

// The probe's contract, in the direction that matters most: it may DROP only on positive
// evidence that the premise is gone, and must pass through everything it does not own.
func TestResourceWarnProbe_PassesThroughWhatItDoesNotOwn(t *testing.T) {
	for _, line := range []string{
		hqwake.Line(hqwake.ClassSelfRotate, "ctx 80%", "over: ctx"),
		hqwake.Line(hqwake.ClassStuckWaiting, "sat:0.0 (%7)", "waited 11m"),
		hqwake.Line(hqwake.ClassUsageWarn, "%18", "ctx 86%"),
		"a bare line from nowhere",
	} {
		keep, out := probeForTest(line)
		if !keep || out != line {
			t.Errorf("probe touched a line it does not own: %q → keep=%v out=%q", line, keep, out)
		}
	}
}

// A queued wake is decided at enqueue and delivered later; the revalidation seam is what
// closes that gap. With no probe registered the chain is an identity — the pre-change
// behavior, and the right default: an unclaimed class must never be silently droppable.
func TestRevalidatorChainDefaultsToIdentity(t *testing.T) {
	line := hqwake.Line(hqwake.ClassWaiting, "sat:0.0 (%7)", "needs you")
	keep, out := probeForTest(line)
	if !keep || out != line {
		t.Fatalf("an unowned line must pass through unchanged, got keep=%v out=%q", keep, out)
	}
}

// splitWakeTail is what lets a probe re-render one field of a line the queue holds only
// as text. Getting it wrong would corrupt a delivered wake, so both shapes are pinned.
func TestSplitWakeTail(t *testing.T) {
	line := hqwake.Line(hqwake.ClassUsageWarn, "sat:0.0 (%74)", "ctx→80% in ~4m")
	head, tail, ok := splitWakeTail(line)
	if !ok || tail != "ctx→80% in ~4m" {
		t.Fatalf("split = (%q, %q, %v)", head, tail, ok)
	}
	if head+hqwake.FieldSep()+tail != line {
		t.Fatal("re-joining the split must reproduce the line exactly")
	}
	// A line with no field at all is not something to rewrite.
	if _, _, ok := splitWakeTail("» ▸ gtmux·usage·warn  sat:0.0 (%74)"); ok {
		t.Error("a field-less line must not be split")
	}
}

// The probe must fail OPEN on everything it cannot establish. Each case below is a step
// that could go wrong at delivery time, and every one of them delivers rather than
// discards — because the alarm it would discard may be real.
func TestUsageWarnProbe_FailsOpen(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no resume records, no usage snapshots: nothing resolves
	cases := map[string]string{
		"no pane id in the head":  hqwake.Line(hqwake.ClassUsageWarn, "sat:0.0", "ctx 86%"),
		"no session on record":    hqwake.Line(hqwake.ClassUsageWarn, "sat:0.0 (%74)", "ctx 86%"),
		"no field to re-render":   "» ▸ gtmux·usage·warn  sat:0.0 (%74)",
		"a class we do not judge": hqwake.Line(hqwake.ClassWaiting, "sat:0.0 (%74)", "needs you"),
	}
	for name, line := range cases {
		keep, out := usageWarnProbe(line)
		if !keep || out != line {
			t.Errorf("%s: probe must deliver unchanged, got keep=%v out=%q", name, keep, out)
		}
	}
}

// A machine the probe re-reads, with every source read unless a test takes one away.
func sampled(freeGB, usePct int, warn string) resource.Machine {
	return resource.Machine{DiskFreeGB: freeGB, DiskUsePct: usePct, MemTier: "normal", NCPU: 8,
		Battery: &resource.Battery{}, Warn: warn,
		Read: resource.Readings{Disk: true, Memory: true, Load: true, Battery: true}}
}

// unread is a healthy-looking re-read with one source that gave no reading.
func unread(drop func(m *resource.Machine)) resource.Machine {
	m := sampled(100, 50, "")
	drop(&m)
	return m
}

func answerResource(t *testing.T, m resource.Machine) {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // default thresholds, never the user's config
	saved := currentResource
	currentResource = func() resource.Report { return resource.Report{Machine: m} }
	t.Cleanup(func() { currentResource = saved })
}

// %12's reproduction (2026-10-06): a queued disk-critical line was delivered as critical
// after the disk had recovered to amber, and an alarm was dropped when every command of
// the re-read failed — zeros, which is what a healthy machine looks like.
func TestResourceWarnProbe_SpeaksTheCurrentConditionAndFailsOpen(t *testing.T) {
	red := resourceWarnLine(resource.Report{Machine: sampled(1, 99, "disk critical · 1GB free")})
	for _, tc := range []struct {
		name     string
		now      resource.Machine
		keep     bool
		wantLine string // "" = the queued line, untouched
	}{
		{"recovered to amber: the current condition", sampled(40, 92, "disk getting low · 40GB free"), true,
			resourceWarnLine(resource.Report{Machine: sampled(40, 92, "disk getting low · 40GB free")})},
		{"recovered fully: dropped", sampled(100, 50, ""), false, ""},
		{"still red: as queued", sampled(2, 99, "disk critical · 2GB free"), true, ""},
		{"every command failed: as queued", resource.Machine{}, true, ""},
		{"pmset failed: as queued", unread(func(m *resource.Machine) { m.Battery, m.Read.Battery = nil, false }), true, ""},
		{"no pressure level: as queued", unread(func(m *resource.Machine) { m.MemTier, m.Read.Memory = "", false }), true, ""},
		// %12's two gaps in 14bb7b22: a core count is not a load reading, and a pmset that
		// exited 0 with nothing readable is not a battery reading.
		{"load average unread, cores known: as queued", unread(func(m *resource.Machine) { m.Read.Load = false }), true, ""},
		{"pmset answer unreadable: as queued", unread(func(m *resource.Machine) { m.Read.Battery = false }), true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			answerResource(t, tc.now)
			keep, out := resourceWarnProbe(red)
			want := tc.wantLine
			if want == "" && tc.keep {
				want = red
			}
			if keep != tc.keep || (keep && out != want) {
				t.Fatalf("keep=%v out=%q, want keep=%v out=%q", keep, out, tc.keep, want)
			}
		})
	}
}

// The same amber, re-read: the queued line is delivered untouched rather than rebuilt.
func TestResourceWarnProbe_LeavesAnUnchangedAmberAlone(t *testing.T) {
	m := sampled(40, 92, "disk getting low · 40GB free")
	answerResource(t, m)
	line := resourceWarnLine(resource.Report{Machine: m})
	if keep, out := resourceWarnProbe(line); !keep || out != line {
		t.Fatalf("keep=%v out=%q, want the queued line", keep, out)
	}
}

// A memory warning re-read before delivery: the same warning, red or amber, with
// whatever reclaim candidates the re-read found.
func memoryShort(tier string, orphans ...resource.Orphan) resource.Report {
	m := sampled(100, 50, "memory warn")
	m.MemTier, m.WarnKey = "warn", resource.WarnMemoryWarn
	if tier == "red" {
		m.MemTier, m.Warn, m.WarnKey = "critical", "memory critical", resource.WarnMemoryCritical
	}
	return resource.Report{Machine: m, Orphans: orphans}
}

// %12's reproduction (2026-10-06): the alarm stood, so the probe delivered the queued
// line whole, including a reclaim candidate that had since gone. The hint is re-read on
// its own; the alarm is never dropped with it.
func TestResourceWarnProbe_ReReadsTheReclaimHint(t *testing.T) {
	gone := resource.Orphan{PID: 4242, Comm: "audit-gone-orphan", RSSMB: 777}
	other := resource.Orphan{PID: 5151, Comm: "another-orphan", RSSMB: 512}
	for _, tier := range []string{"amber", "red"} {
		queued := resourceWarnLine(memoryShort(tier, gone))
		for _, tc := range []struct {
			name string
			now  resource.Report
		}{
			{"the candidate is gone", memoryShort(tier)},
			{"another candidate now", memoryShort(tier, other)},
			{"the same candidate", memoryShort(tier, gone)},
		} {
			t.Run(tier+": "+tc.name, func(t *testing.T) {
				answerResource(t, tc.now.Machine)
				saved := currentResource
				currentResource = func() resource.Report { return tc.now }
				t.Cleanup(func() { currentResource = saved })
				keep, out := resourceWarnProbe(queued)
				if want := resourceWarnLine(tc.now); !keep || out != want {
					t.Fatalf("keep=%v out=%q, want %q", keep, out, want)
				}
			})
		}
	}
}

// The re-read is now red while the queued line says amber for the same resource: the
// queued warning stands (understated), and its hint survives only if the re-read names
// the same candidate.
func TestResourceWarnProbe_KeepsAWorsenedWarningAndOnlyAConfirmedHint(t *testing.T) {
	gone := resource.Orphan{PID: 4242, Comm: "audit-gone-orphan", RSSMB: 777}
	queued := resourceWarnLine(memoryShort("amber", gone))
	head, _, _ := splitWakeTail(queued)
	for _, tc := range []struct {
		name     string
		now      resource.Report
		wantHint bool
	}{
		{"the candidate is gone", memoryShort("red"), false},
		{"the same candidate", memoryShort("red", gone), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			answerResource(t, tc.now.Machine)
			saved := currentResource
			currentResource = func() resource.Report { return tc.now }
			t.Cleanup(func() { currentResource = saved })
			keep, out := resourceWarnProbe(queued)
			want := head + hqwake.FieldSep() + "memory warn"
			if tc.wantHint {
				want = queued
			}
			if !keep || out != want {
				t.Fatalf("keep=%v out=%q, want %q", keep, out, want)
			}
		})
	}
}
