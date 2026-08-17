package hq

import (
	"os"
	"strings"
	"testing"
)

func TestKnowledgeLedgerRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ops := []knowledgeOp{
		{Op: knowledgeOpAdd, ID: "pitfalls/wrangler-tls", Topic: "pitfalls",
			Title: "wrangler TLS-resets from the office; retry", At: 100, Seq: 6650,
			Pane: "%21", Task: "t-9", Capture: "pitfalls/wrangler-tls", Seqs: []int64{6650, 6651}},
		{Op: knowledgeOpRetire, ID: "pitfalls/wrangler-tls", Topic: "pitfalls",
			At: 200, Seq: 7000, Why: "the office network was fixed"},
	}
	for _, op := range ops {
		if err := appendKnowledgeOp(op); err != nil {
			t.Fatalf("append %s: %v", op.Op, err)
		}
	}
	got, err := readKnowledgeOps()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("read %d ops, want 2", len(got))
	}
	if got[0].V != knowledgeSchemaV {
		t.Errorf("schema version not stamped: %+v", got[0])
	}
	if got[0].Pane != "%21" || got[0].Task != "t-9" || len(got[0].Seqs) != 2 {
		t.Errorf("provenance did not survive the round-trip: %+v", got[0])
	}
}

func TestKnowledgeFold(t *testing.T) {
	add := func(id, title string) knowledgeOp {
		return knowledgeOp{Op: knowledgeOpAdd, ID: id, Topic: "pitfalls", Title: title}
	}
	ops := []knowledgeOp{
		add("pitfalls/a", "a v1"),
		add("pitfalls/b", "b"),
		// supersede a with a NEW id (retitled)
		{Op: knowledgeOpSupersede, ID: "pitfalls/a-two", Topic: "pitfalls",
			Title: "a v2", Supersedes: "pitfalls/a"},
		// supersede in place (same title → same id)
		{Op: knowledgeOpSupersede, ID: "pitfalls/b", Topic: "pitfalls",
			Title: "b", Supersedes: "pitfalls/b", Body: "sharper"},
		add("pitfalls/c", "c"),
		{Op: knowledgeOpRetire, ID: "pitfalls/c", Topic: "pitfalls", Why: "dead"},
	}
	live := foldKnowledge(ops)
	if len(live) != 2 {
		t.Fatalf("live = %d entries, want 2 (a-two, b)", len(live))
	}
	if live[0].ID != "pitfalls/a-two" || live[0].Title != "a v2" {
		t.Errorf("supersede-with-retitle: live[0] = %+v", live[0])
	}
	if live[1].ID != "pitfalls/b" || live[1].Body != "sharper" {
		t.Errorf("supersede-in-place must keep position and take the new content: %+v", live[1])
	}
	if _, ok := findLive(live, "pitfalls/c"); ok {
		t.Error("a retired entry is dead")
	}
	if _, ok := findLive(live, "pitfalls/a"); ok {
		t.Error("a superseded predecessor is dead")
	}
}

func TestKnowledgeBoundsRefuseLoudly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cases := []struct {
		name string
		op   knowledgeOp
		want string
	}{
		{"multi-line title", knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/x", Topic: "pitfalls",
			Title: "line one\nline two"}, "single line"},
		{"title over budget", knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/x", Topic: "pitfalls",
			Title: strings.Repeat("t", knowledgeTitleMax+1)}, "limit is 200"},
		{"body over budget", knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/x", Topic: "pitfalls",
			Title: "t", Body: strings.Repeat("b", knowledgeBodyMax+1)}, "limit is 8192"},
		{"why over budget", knowledgeOp{Op: knowledgeOpRetire, ID: "pitfalls/x", Topic: "pitfalls",
			Why: strings.Repeat("w", knowledgeWhyMax+1)}, "limit is 300"},
	}
	for _, c := range cases {
		err := appendKnowledgeOp(c.op)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want refusal naming %q", c.name, err, c.want)
		}
	}
	// Nothing over-budget reached the disk.
	if _, err := os.Stat(knowledgeLedgerPath()); !os.IsNotExist(err) {
		t.Fatal("a refused op must append nothing")
	}
}

func TestKnowledgeMalformedLineIsSkipped(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := appendKnowledgeOp(knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/a",
		Topic: "pitfalls", Title: "a"}); err != nil {
		t.Fatal(err)
	}
	f, _ := os.OpenFile(knowledgeLedgerPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("{not json\n\n")
	_ = f.Close()
	if err := appendKnowledgeOp(knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/b",
		Topic: "pitfalls", Title: "b"}); err != nil {
		t.Fatal(err)
	}
	ops, err := readKnowledgeOps()
	if err != nil || len(ops) != 2 {
		t.Fatalf("one bad line must not take the ledger hostage: ops=%d err=%v", len(ops), err)
	}
}

func TestKnowledgeTopics(t *testing.T) {
	for _, ok := range append(append([]string{}, captureTopics...), "environment") {
		if !validKnowledgeTopic(ok) {
			t.Errorf("topic %q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "README", "notes", "board"} {
		if validKnowledgeTopic(bad) {
			t.Errorf("topic %q should be invalid", bad)
		}
	}
}

// The promotion lifecycle in the fold: promote opens, land closes, a re-promote
// re-opens, and a supersede is re-judged from scratch (hq-promotion-exit).
func TestKnowledgePromotionLifecycleFold(t *testing.T) {
	add := knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/a", Topic: "pitfalls", Title: "a"}
	promote := knowledgeOp{Op: knowledgeOpPromote, ID: "pitfalls/a", Topic: "pitfalls",
		At: 100, Why: "holds on any machine; changes the seeded playbook", Target: "supervisor-agent spec"}
	land := knowledgeOp{Op: knowledgeOpLand, ID: "pitfalls/a", Topic: "pitfalls", At: 200, Ref: "#812"}

	live := foldKnowledge([]knowledgeOp{add, promote})
	e, _ := findLive(live, "pitfalls/a")
	if !promotionPending(e) || e.PromoteWhy == "" || e.PromoteTarget == "" || e.PromotedAt != 100 {
		t.Fatalf("promote must open the lifecycle with why/target: %+v", e)
	}

	live = foldKnowledge([]knowledgeOp{add, promote, land})
	e, _ = findLive(live, "pitfalls/a")
	if promotionPending(e) || e.LandedRef != "#812" || e.LandedAt != 200 {
		t.Fatalf("land must close with the ref: %+v", e)
	}

	// A re-promote re-opens a landed entry (the lesson evolved).
	rePromote := promote
	rePromote.At, rePromote.Why = 300, "evolved"
	live = foldKnowledge([]knowledgeOp{add, promote, land, rePromote})
	e, _ = findLive(live, "pitfalls/a")
	if !promotionPending(e) || e.PromotedAt != 300 || e.LandedRef != "" {
		t.Fatalf("a re-promote must re-open and clear the landed pair: %+v", e)
	}

	// A supersede does NOT inherit promotion state — the successor is re-judged.
	sup := knowledgeOp{Op: knowledgeOpSupersede, ID: "pitfalls/a-two", Topic: "pitfalls",
		Title: "a two", Supersedes: "pitfalls/a"}
	live = foldKnowledge([]knowledgeOp{add, promote, sup})
	e, _ = findLive(live, "pitfalls/a-two")
	if e.PromotedAt != 0 || promotionPending(e) {
		t.Fatalf("a successor carries no promotion state: %+v", e)
	}

	// pendingPromotions reports the queue and the oldest.
	addB := knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/b", Topic: "pitfalls", Title: "b"}
	promoteB := knowledgeOp{Op: knowledgeOpPromote, ID: "pitfalls/b", Topic: "pitfalls", At: 50, Why: "w"}
	pending, oldest := pendingPromotions(foldKnowledge([]knowledgeOp{add, promote, addB, promoteB}))
	if len(pending) != 2 || oldest != 50 {
		t.Fatalf("pending=%d oldest=%d, want 2/50", len(pending), oldest)
	}
}

func TestKnowledgePromotionBoundsRefuseLoudly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	long := strings.Repeat("t", knowledgeTargetMax+1)
	err := appendKnowledgeOp(knowledgeOp{Op: knowledgeOpPromote, ID: "pitfalls/a",
		Topic: "pitfalls", Why: "w", Target: long})
	if err == nil || !strings.Contains(err.Error(), "limit is 200") {
		t.Fatalf("over-budget target must refuse loudly, got %v", err)
	}
	err = appendKnowledgeOp(knowledgeOp{Op: knowledgeOpLand, ID: "pitfalls/a",
		Topic: "pitfalls", Ref: strings.Repeat("r", knowledgeRefMax+1)})
	if err == nil || !strings.Contains(err.Error(), "limit is 200") {
		t.Fatalf("over-budget ref must refuse loudly, got %v", err)
	}
}
