package hq

import (
	"encoding/json"
	"github.com/chenchaoyi/gtmux/internal/knowledge"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/mine"
)

// Nothing here opens a real transcript: the sensor is tested at its gates, and the spool
// mapping on hand-made candidates. HOME is a temp dir in every test.

func TestMinedCandidateLandsInTheSpoolShape(t *testing.T) {
	c := mine.Candidate{Kind: mine.KindCorrection, ID: "abc123def456", At: 10, Session: "s1", Project: "demo",
		Line: "这个不对，重新做", Context: "…shipped it."}
	cc := spoolFromMined(c, 20, 99)
	if cc.Topic != "corrections" || cc.Key != "corrections/mined-abc123def456" || cc.Lesson != c.Line ||
		cc.Source != captureSourceTranscript || cc.Context != c.Context || cc.Session != "s1" || cc.Project != "demo" || cc.Seq != 99 {
		t.Fatalf("spool shape: %+v", cc)
	}
	e := spoolFromMined(mine.Candidate{Kind: mine.KindError, Line: "bash: wrangler: command not found", Count: 5, Sessions: 3}, 20, 99)
	if e.Topic != "pitfalls" || !strings.HasPrefix(e.Key, "pitfalls/mined-") || !strings.Contains(e.Lesson, "×5") || e.Count != 5 {
		t.Fatalf("error spool shape: %+v", e)
	}
	// The additive fields must round-trip through the spool file and stay omitted on a
	// plain capture line — a surface reading the queue sees who queued what.
	b, _ := json.Marshal(cc)
	if !strings.Contains(string(b), `"source":"transcript"`) {
		t.Fatalf("source not serialized: %s", b)
	}
	b, _ = json.Marshal(knowledge.Candidate{Topic: "pitfalls", Lesson: "x"})
	if strings.Contains(string(b), "source") || strings.Contains(string(b), "context") {
		t.Fatalf("a capture line grew fields: %s", b)
	}
}

func TestMineSensorGates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	// No HQ home → no pass, no ledger.
	mineSensor(now)
	if mine.LastPassAt(mineDir()) != 0 {
		t.Fatal("ran without an HQ home")
	}
	if err := os.MkdirAll(hqKnowledgeDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	// With a home: a pass runs (against no logs — HOME is empty) and stamps the ledger.
	mineSensor(now)
	first := mine.LastPassAt(mineDir())
	if first == 0 {
		t.Fatal("did not run with an HQ home")
	}
	// Inside the interval → no second pass.
	mineSensor(now + 3600)
	if mine.LastPassAt(mineDir()) != first {
		t.Fatal("ran again inside the interval")
	}
	// Past the interval → runs again.
	mineSensor(now + 25*3600)
	if mine.LastPassAt(mineDir()) == first {
		t.Fatal("did not run after the interval")
	}
	// Interval 0 disables the daily pass entirely.
	cfg := filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "config.json")
	os.MkdirAll(filepath.Dir(cfg), 0o755)
	os.WriteFile(cfg, []byte(`{"hqWake":{"mineIntervalHours":0}}`), 0o644)
	last := mine.LastPassAt(mineDir())
	mineSensor(now + 100*3600)
	if mine.LastPassAt(mineDir()) != last {
		t.Fatal("interval 0 must disable the sensor")
	}
}

func TestParseSinceArg(t *testing.T) {
	for in, want := range map[string]time.Duration{"all": 0, "7d": 7 * 24 * time.Hour, "36h": 36 * time.Hour, "30": 30 * 24 * time.Hour} {
		d, ok := parseSinceArg(in)
		if !ok || d != want {
			t.Errorf("%s → %v %v", in, d, ok)
		}
	}
	for _, bad := range []string{"", "0d", "-3d", "x"} {
		if _, ok := parseSinceArg(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestPlaybookTeachesTranscriptLeadsInBothLanguages(t *testing.T) {
	for _, tc := range []struct{ name, body, mark string }{
		{"en", hqInstructions, "TRANSCRIPT MINER"},
		{"zh", hqInstructionsZH, "会话采矿器"},
	} {
		if !strings.Contains(tc.body, tc.mark) || !strings.Contains(tc.body, "knowledge mine --status") {
			t.Errorf("%s playbook does not teach the transcript leads", tc.name)
		}
	}
	if hqPlaybookVersion < 37 {
		t.Errorf("hqPlaybookVersion = %d; the transcript-lead teaching shipped in 37", hqPlaybookVersion)
	}
}
