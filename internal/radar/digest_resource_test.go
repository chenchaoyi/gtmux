package radar

import (
	"encoding/json"
	"errors"
	"testing"
)

// One `ps -o pid=,ppid=,cputime=,rss=,%cpu=,command=` line: the numbers in their columns,
// the command whatever is left, and a number that will not parse is no signal, not a lost
// process.
func TestParseProcLine(t *testing.T) {
	pid, info, ok := parseProcLine("  4242     1   0:01.25  20480  12.5 /usr/local/bin/claude --resume x")
	if !ok || pid != 4242 || info.ppid != 1 || info.rssKB != 20480 || info.pcpu != 12.5 ||
		info.cpu != 1.25 || info.command != "/usr/local/bin/claude --resume x" {
		t.Fatalf("parsed (%d, %+v, %v)", pid, info, ok)
	}
	if _, info, ok := parseProcLine("7 1 0:00.00 - 0,3 sh"); !ok || info.rssKB != 0 || info.pcpu != 0 || info.command != "sh" {
		t.Fatalf("unparsable numbers: (%+v, %v)", info, ok)
	}
	for _, bad := range []string{"", "1 2 0:00 10 0.0", "x 1 0:00 10 0.0 sh"} {
		if _, _, ok := parseProcLine(bad); ok {
			t.Errorf("parseProcLine(%q) accepted", bad)
		}
	}
}

// A digest row carries its pane's process tree, summed, as `gtmux resource` attributes it
// (resource-watch "Per-agent resource attribution"; the field was promised and never
// delivered). Read from the radar's own process table: no second ps.
func TestDigestRowCarriesItsPanesResourceUse(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	lines := []string{
		paneLine("%1", "w", "0", "0", "✳ ready", "claude", 1700000000, 900001, "/tmp/nope"),
		paneLine("%2", "w", "0", "1", "✳ ready", "claude", 1700000000, 900002, "/tmp/nope"),
	}
	procs := map[int]procInfo{
		900001: {ppid: 1, rssKB: 1024, pcpu: 0.5, command: "claude"},
		910001: {ppid: 900001, rssKB: 204800, pcpu: 12.46, command: "node tool.js"},
		920001: {ppid: 910001, rssKB: 10240, pcpu: 1.0, command: "rg x"},
		// %2's root is not in the table (it exited between the scan and the read).
	}
	var rows []DigestRow
	withFixture(t, lines, func() {
		procSnapshot = func() map[int]procInfo { return procs }
		rows = GatherDigest()
	})
	got := map[string]DigestRow{}
	for _, r := range rows {
		got[r.PaneID] = r
	}
	if r := got["%1"]; r.RSSMB != 211 || r.CPU != 14 {
		t.Errorf("%%1 = rss %d MB, cpu %v; want 211 MB, 14.0 (1024+204800+10240 KB; 0.5+12.46+1.0 %%)", r.RSSMB, r.CPU)
	}
	if r := got["%2"]; r.RSSMB != 0 || r.CPU != 0 {
		t.Errorf("%%2 = rss %d, cpu %v; want none", r.RSSMB, r.CPU)
	}
	// Additive and omitted when absent: a consumer that never heard of them sees no key.
	b, _ := json.Marshal(got["%2"])
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if _, ok := m["rss_mb"]; ok {
		t.Errorf("a row with no figure carries rss_mb: %s", b)
	}
}

// One digest, one ps, whether the read works or fails. A failed read is not cached, so
// the digest asking for the table again ran a second ps (%12's review of 59bb0a21); it
// now uses the gather's own read, and a failed one leaves the rows without figures.
func TestDigestRunsOnePS(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	lines := []string{paneLine("%1", "w", "0", "0", "✳ ready", "claude", 1700000000, 900001, "/tmp/nope")}
	for _, tc := range []struct {
		name string
		out  []byte
		err  error
		rss  int
	}{
		{"ps fails", nil, errors.New("ps wedged"), 0},
		{"ps answers", []byte("900001 1 0:00.10 4096 1.5 claude\n"), nil, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetProcCache()
			calls := 0
			orig := boundedPS
			boundedPS = func(...string) ([]byte, error) { calls++; return tc.out, tc.err }
			defer func() { boundedPS = orig }()
			var rows []DigestRow
			withFixture(t, lines, func() {
				procSnapshot = snapshotProcs // the real read, over the stand-in ps
				rows = GatherDigest()
			})
			if calls != 1 {
				t.Errorf("one digest ran ps %d times, want 1", calls)
			}
			if len(rows) != 1 || rows[0].RSSMB != tc.rss {
				t.Errorf("rows = %+v, want one row with rss %d MB", rows, tc.rss)
			}
		})
	}
}
