package knowledge

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The three axes (hq-knowledge-engine phase 2). Everything here runs on a synthetic
// ledger in a temp HOME.

func writeOps(t *testing.T, ops ...knowledgeOp) {
	t.Helper()
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(knowledgeLedgerPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, op := range ops {
		b, _ := json.Marshal(op)
		f.Write(append(b, '\n'))
	}
}

func mustLive(t *testing.T, id string) knowledgeOp {
	t.Helper()
	live, err := liveKnowledge()
	if err != nil {
		t.Fatal(err)
	}
	op, ok := findLive(live, id)
	if !ok {
		t.Fatalf("no live entry %s", id)
	}
	return op
}

// A v1 record says nothing about its kind; the fold fills it from the topic and marks
// it ASSUMED, and a corrections entry reads as provenance `correction`. The file is not
// touched.
func TestLegacyRecordsAreMigratedAtReadOnly(t *testing.T) {
	asHQ(t)
	writeOps(t,
		knowledgeOp{V: 1, Op: knowledgeOpAdd, ID: "corrections/x", Topic: "corrections", Title: "x", At: 10, Seq: 1},
		knowledgeOp{V: 1, Op: knowledgeOpAdd, ID: "workflows/y", Topic: "workflows", Title: "y", At: 11, Seq: 2, Capture: "workflows/y"},
		knowledgeOp{V: 1, Op: knowledgeOpTopic, ID: "datasets", Title: "d", At: 12, Seq: 3},
		knowledgeOp{V: 1, Op: knowledgeOpAdd, ID: "datasets/z", Topic: "datasets", Title: "z", At: 13, Seq: 4},
	)
	before, _ := os.ReadFile(knowledgeLedgerPath())
	x, y, z := mustLive(t, "corrections/x"), mustLive(t, "workflows/y"), mustLive(t, "datasets/z")
	if x.Kind != KindJudgment || !x.KindAssumed || x.Provenance != ProvCorrection || x.Hits != 1 {
		t.Fatalf("corrections → judgment?/correction: %+v", x)
	}
	if y.Kind != KindHowto || !y.KindAssumed || y.Provenance != ProvCapture {
		t.Fatalf("workflows with a capture → howto?/capture: %+v", y)
	}
	if z.Kind != KindFacts || !z.KindAssumed || z.Provenance != ProvSelf {
		t.Fatalf("a declared topic → facts?/self: %+v", z)
	}
	after, _ := os.ReadFile(knowledgeLedgerPath())
	if string(before) != string(after) {
		t.Fatal("reading must not rewrite the ledger")
	}
}

// An explicit --kind is a judgement, not a guess; a stated topic without --kind still
// gets the topic's kind, but NOT assumed — HQ chose the topic knowing what it holds.
func TestAddCarriesTheAxes(t *testing.T) {
	asHQ(t)
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "a", "--kind", "judgment", "--tags", "release,ci", "--provenance", "correction"}); rc != 0 {
		t.Fatal("add with axes failed")
	}
	a := mustLive(t, "pitfalls/a")
	if a.Kind != KindJudgment || a.KindAssumed || a.Provenance != ProvCorrection || len(a.Tags) != 2 || a.Hits != 1 {
		t.Fatalf("axes lost: %+v", a)
	}
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "b"}); rc != 0 {
		t.Fatal("plain add failed")
	}
	b := mustLive(t, "pitfalls/b")
	if b.Kind != KindPitfalls || b.KindAssumed || b.Provenance != ProvSelf {
		t.Fatalf("a stated topic implies its kind without being a guess: %+v", b)
	}
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "c", "--kind", "rumour"}); rc == 0 {
		t.Fatal("an unknown kind must be refused")
	}
}

// Consuming mined candidates makes the entry `mined`, and its hit count is what the
// candidates carried (a recurring error's count, one per correction lead).
func TestAddFromMinedCandidatesCountsTheObservations(t *testing.T) {
	asHQ(t)
	for _, c := range []Candidate{
		{At: 1, Topic: "pitfalls", Key: "pitfalls/mined-bash-x", Lesson: "bash: x (×5, 3 sessions)", Seq: 5, Source: "transcript", Count: 5},
		{At: 2, Topic: "corrections", Key: "corrections/mined-a1", Lesson: "不对", Seq: 6, Source: "transcript"},
	} {
		if err := AppendCandidate(c); err != nil {
			t.Fatal(err)
		}
	}
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "x is missing", "--capture", "pitfalls/mined-bash-x,corrections/mined-a1"}); rc != 0 {
		t.Fatal("add --capture failed")
	}
	e := mustLive(t, "pitfalls/x-is-missing")
	if e.Provenance != ProvMined || e.Hits != 6 {
		t.Fatalf("mined ×6 expected: %+v", e)
	}
}

// A hypothesis renders in its own section and never reaches the machine file; confirm
// moves it into the main list.
func TestHypothesisIsShelvedNotDistributed(t *testing.T) {
	asHQ(t)
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "maybe", "--hypothesis"}); rc != 0 {
		t.Fatal("add --hypothesis failed")
	}
	writeOps(t, knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/sure", Topic: "pitfalls", Title: "sure", At: 5, Seq: 9, Kind: KindPitfalls, Audience: AudienceMachine})
	if rc := CmdKnowledge([]string{"render"}); rc != 0 {
		t.Fatal("render failed")
	}
	topic, _ := os.ReadFile(topicPath("pitfalls"))
	s := string(topic)
	i, j := strings.Index(s, "## unverified"), strings.Index(s, "**maybe**")
	if i < 0 || j < i {
		t.Fatalf("hypothesis must sit under the unverified section:\n%s", s)
	}
	machine, _ := os.ReadFile(MachinePath())
	if strings.Contains(string(machine), "maybe") || !strings.Contains(string(machine), "**sure**") {
		t.Fatalf("machine file must carry the audience entry and never the hypothesis:\n%s", machine)
	}
	if rc := CmdKnowledge([]string{"confirm", "pitfalls/maybe"}); rc != 0 {
		t.Fatal("confirm failed")
	}
	if e := mustLive(t, "pitfalls/maybe"); e.Status != "" {
		t.Fatalf("confirm must clear the status: %+v", e)
	}
	if rc := CmdKnowledge([]string{"confirm", "pitfalls/maybe"}); rc == 0 {
		t.Fatal("confirming a live entry must refuse")
	}
}

// hit adds to the count and turns a self-observed lesson into a recurrence; the
// supersede inherits count and kind; the kind verb clears the assumed mark.
func TestHitsAccumulateAndSurviveSupersede(t *testing.T) {
	asHQ(t)
	writeOps(t, knowledgeOp{V: 1, Op: knowledgeOpAdd, ID: "pitfalls/p", Topic: "pitfalls", Title: "p", At: 1, Seq: 1})
	if rc := CmdKnowledge([]string{"hit", "pitfalls/p", "--n", "3", "--why", "again in %4"}); rc != 0 {
		t.Fatal("hit failed")
	}
	p := mustLive(t, "pitfalls/p")
	if p.Hits != 4 || p.Provenance != ProvRecurrence || p.HitLast == 0 {
		t.Fatalf("1 + 3 hits, recurrence: %+v", p)
	}
	if rc := CmdKnowledge([]string{"supersede", "pitfalls/p", "--title", "p, sharper"}); rc != 0 {
		t.Fatal("supersede failed")
	}
	q := mustLive(t, "pitfalls/p-sharper")
	if q.Hits != 4 || q.Kind != KindPitfalls || !q.KindAssumed || q.Provenance != ProvRecurrence {
		t.Fatalf("the axes must travel with the lesson: %+v", q)
	}
	if rc := CmdKnowledge([]string{"kind", "pitfalls/p-sharper", "judgment"}); rc != 0 {
		t.Fatal("kind failed")
	}
	if r := mustLive(t, "pitfalls/p-sharper"); r.Kind != KindJudgment || r.KindAssumed {
		t.Fatalf("kind must confirm: %+v", r)
	}
	// The trail is in the render.
	topic, _ := os.ReadFile(topicPath("pitfalls"))
	if !strings.Contains(string(topic), "judgment · from recurrence ×4") {
		t.Fatalf("footer must show the axes:\n%s", topic)
	}
}

// The first v2 write backs the v1 ledger up, once.
func TestFirstV2WriteBacksUpTheV1Ledger(t *testing.T) {
	asHQ(t)
	writeOps(t, knowledgeOp{V: 1, Op: knowledgeOpAdd, ID: "pitfalls/old", Topic: "pitfalls", Title: "old", At: 1, Seq: 1})
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "new"}); rc != 0 {
		t.Fatal("add failed")
	}
	bak, err := os.ReadFile(knowledgeLedgerPath() + ".bak-v1")
	if err != nil || !strings.Contains(string(bak), `"id":"pitfalls/old"`) || strings.Contains(string(bak), `"v":2`) {
		t.Fatalf("backup must hold exactly the v1 ledger: %v %s", err, bak)
	}
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "newer"}); rc != 0 {
		t.Fatal("second add failed")
	}
	again, _ := os.ReadFile(knowledgeLedgerPath() + ".bak-v1")
	if string(again) != string(bak) {
		t.Fatal("the backup is written once")
	}
}

// The miner's feedback path: a signature the ledger already holds bumps that entry.
func TestHitByCaptureKeyFindsTheFiledEntry(t *testing.T) {
	asHQ(t)
	writeOps(t, knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/x", Topic: "pitfalls", Title: "x", At: 1, Seq: 1,
		Kind: KindPitfalls, Provenance: ProvMined, Hits: 5, Capture: "pitfalls/mined-bash-x,corrections/mined-a1"})
	t.Chdir(t.TempDir()) // not the HQ home: the miner runs from serve
	id, ok, err := HitByCaptureKey("pitfalls/mined-bash-x", 2, 100)
	if err != nil || !ok || id != "pitfalls/x" {
		t.Fatalf("hit by key: %v %v %s", err, ok, id)
	}
	if e := mustLive(t, "pitfalls/x"); e.Hits != 7 || e.HitLast != 100 {
		t.Fatalf("5 + 2: %+v", e)
	}
	if _, ok, _ := HitByCaptureKey("pitfalls/mined-nobody-filed", 1, 101); ok {
		t.Fatal("a key nobody filed has no entry to bump")
	}
}

// The API rows carry the axes.
func TestIndexRowsCarryTheAxes(t *testing.T) {
	asHQ(t)
	writeOps(t, knowledgeOp{V: 1, Op: knowledgeOpAdd, ID: "corrections/c", Topic: "corrections", Title: "c", At: 1, Seq: 1})
	idx := KnowledgeIndex(10)
	if len(idx.Entries) != 1 {
		t.Fatal("one entry")
	}
	r := idx.Entries[0]
	if r.Kind != KindJudgment || !r.KindAssumed || r.Provenance != ProvCorrection || r.Hits != 1 {
		t.Fatalf("row: %+v", r)
	}
}
