package knowledge

import (
	"os"
	"strings"
	"testing"
)

// Lint and neighbours (phase 4) on synthetic ledgers, plus the two ACE constraints:
// nothing rewrites the ledger, and a superseded text stays readable.

func findings(rep LintReport, check string) []Finding {
	var out []Finding
	for _, f := range rep.Findings {
		if f.Check == check {
			out = append(out, f)
		}
	}
	return out
}

func TestLintFindsEachShape(t *testing.T) {
	asHQ(t)
	const now = int64(1_800_000_000)
	writeOps(t,
		knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/a", Topic: "pitfalls", Title: "retry the wrangler deploy", Body: "same as [[pitfalls/b]] and [[c-old]] and [[nowhere]]", At: 1, Seq: 1, Kind: KindPitfalls},
		knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/b", Topic: "pitfalls", Title: "retry the wrangler deploy again", Body: "x", At: 2, Seq: 2, Kind: KindPitfalls},
		knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/c-old", Topic: "pitfalls", Title: "c old", Body: "x", At: 3, Seq: 3, Kind: KindPitfalls},
		knowledgeOp{V: 2, Op: knowledgeOpSupersede, ID: "pitfalls/c-new", Supersedes: "pitfalls/c-old", Topic: "pitfalls", Title: "c new", Body: "x", At: 4, Seq: 4, Kind: KindPitfalls},
		knowledgeOp{V: 1, Op: knowledgeOpAdd, ID: "workflows/legacy", Topic: "workflows", Title: "legacy", Body: "no links", At: 5, Seq: 5},
		knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/h", Topic: "pitfalls", Title: "maybe", Body: "y", At: now - 40*86400, Seq: 6, Kind: KindPitfalls, Status: StatusHypothesis},
		knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/p", Topic: "pitfalls", Title: "promoted long ago", Body: "z", At: 7, Seq: 7, Kind: KindPitfalls},
		knowledgeOp{V: 2, Op: knowledgeOpPromote, ID: "pitfalls/p", At: now - 20*86400, Seq: 8, Why: "w", Audience: AudienceMachine},
		knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/e", Topic: "pitfalls", Title: "the product should refuse a send into a busy pane", Body: "issue material", At: 9, Seq: 9, Kind: KindPitfalls},
		knowledgeOp{V: 2, Op: knowledgeOpPromote, ID: "pitfalls/e", At: now - 40*86400, Seq: 10, Why: "w", Audience: AudienceEveryone},
		knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/n", Topic: "pitfalls", Title: "no audience was chosen here", Body: "q", At: 11, Seq: 11, Kind: KindPitfalls},
		knowledgeOp{V: 2, Op: knowledgeOpPromote, ID: "pitfalls/n", At: now - 1, Seq: 12, Why: "w"},
	)
	rep, err := Lint(now)
	if err != nil {
		t.Fatal(err)
	}
	// broken: [[nowhere]] only; [[c-old]] resolves through the supersede chain as outdated.
	if b := findings(rep, "broken-link"); len(b) != 1 || !strings.Contains(b[0].Detail, "[[nowhere]]") {
		t.Fatalf("broken: %+v", b)
	}
	if o := findings(rep, "outdated-link"); len(o) != 1 || !strings.Contains(o[0].Detail, "now pitfalls/c-new") {
		t.Fatalf("outdated: %+v", o)
	}
	// orphans: everything with no link in or out — not a, not b (linked from a), not c-new
	// (linked via the chain).
	orphanIDs := map[string]bool{}
	for _, f := range findings(rep, "orphan") {
		orphanIDs[f.ID] = true
	}
	for id, want := range map[string]bool{"pitfalls/a": false, "pitfalls/b": false, "pitfalls/c-new": false, "workflows/legacy": true, "pitfalls/h": true} {
		if orphanIDs[id] != want {
			t.Errorf("orphan %s = %v, want %v (%v)", id, orphanIDs[id], want, orphanIDs)
		}
	}
	// near-duplicate: a and b share a title.
	if d := findings(rep, "near-duplicate"); len(d) != 1 || d[0].ID != "pitfalls/a" || !strings.Contains(d[0].Detail, "pitfalls/b") {
		t.Fatalf("duplicate: %+v", d)
	}
	// assumed-kind: only the v1 record.
	if a := findings(rep, "assumed-kind"); len(a) != 1 || a[0].ID != "workflows/legacy" {
		t.Fatalf("assumed: %+v", a)
	}
	// stale: the old hypothesis, the old machine promotion, the audience-less one — NOT
	// the older everyone promotion.
	staleIDs := map[string]bool{}
	for _, f := range findings(rep, "stale") {
		staleIDs[f.ID] = true
	}
	for id, want := range map[string]bool{"pitfalls/h": true, "pitfalls/p": true, "pitfalls/n": true, "pitfalls/e": false} {
		if staleIDs[id] != want {
			t.Errorf("stale %s = %v, want %v", id, staleIDs[id], want)
		}
	}
	if !strings.HasPrefix(rep.Summary(), "lint: 8 entries · ") {
		t.Fatalf("summary: %q", rep.Summary())
	}
	// Lint never writes.
	before, _ := os.ReadFile(knowledgeLedgerPath())
	Lint(now)
	after, _ := os.ReadFile(knowledgeLedgerPath())
	if string(before) != string(after) {
		t.Fatal("lint must not touch the ledger")
	}
}

func TestLinksIgnoreShellAndSpaces(t *testing.T) {
	got := links("see [[pitfalls/x]] and [[ $- == *i* ]] and [[a b]] and [[c-d]]")
	if len(got) != 2 || got[0] != "pitfalls/x" || got[1] != "c-d" {
		t.Fatalf("links: %v", got)
	}
}

func TestNeighboursRankByOverlapAndKind(t *testing.T) {
	live := []knowledgeOp{
		{ID: "pitfalls/a", Kind: KindPitfalls, Title: "wrangler deploy TLS resets from the office network", Body: "retry once"},
		{ID: "howto/b", Kind: KindHowto, Title: "wrangler deploy from the office network", Body: "retry once, then cellular"},
		{ID: "facts/c", Kind: KindFacts, Title: "the eval set lives under data", Body: "regenerate with make"},
		{ID: "pitfalls/d", Kind: KindPitfalls, Title: "模拟器回收建议把内存和磁盘混在一起", Body: "磁盘要删文件，内存要杀进程"},
	}
	near := neighboursOf(live, "wrangler TLS reset office network deploy", KindPitfalls, "", 3)
	if len(near) < 2 || near[0].ID != "pitfalls/a" || near[1].ID != "howto/b" {
		t.Fatalf("same kind first, then overlap: %+v", near)
	}
	for _, n := range near {
		if n.ID == "facts/c" {
			t.Fatal("an unrelated entry must not appear")
		}
	}
	// CJK bigrams carry meaning without spaces.
	near = neighboursOf(live, "回收建议把内存当磁盘", "", "", 1)
	if len(near) != 1 || near[0].ID != "pitfalls/d" {
		t.Fatalf("CJK overlap: %+v", near)
	}
	// Exclude self.
	if near := neighboursOf(live, entryText(live[0]), KindPitfalls, "pitfalls/a", 3); len(near) > 0 && near[0].ID == "pitfalls/a" {
		t.Fatal("an entry is not its own neighbour")
	}
}

func TestCandidatesGroupIntoFamilies(t *testing.T) {
	cands := []Candidate{
		{Key: "corrections/mined-1", Lesson: "你没有判断目标在忙就派了新任务", Context: "派活"},
		{Key: "pitfalls/mined-x", Lesson: "bash: wrangler: command not found (×5)", Context: ""},
		{Key: "corrections/mined-2", Lesson: "目标还在忙，你又派了一个任务过去", Context: "派活"},
		{Key: "corrections/mined-3", Lesson: "这个 What's New 翻译腔太浓", Context: "文案"},
	}
	groups := groupCandidates(cands)
	if len(groups) != 3 {
		t.Fatalf("want 3 families, got %d: %+v", len(groups), groups)
	}
	if len(groups[0]) != 2 || groups[0][0].Key != "corrections/mined-1" || groups[0][1].Key != "corrections/mined-2" {
		t.Fatalf("the two dispatch corrections are one family, in pool order: %+v", groups[0])
	}
}

// ACE constraint: a shortened supersede never destroys the longer text.
func TestSupersededTextStaysReadable(t *testing.T) {
	asHQ(t)
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "the long form", "--body-file", "-"}); rc == 0 {
		t.Log("add with stdin body may refuse in tests; using the ledger directly")
	}
	writeOps(t, knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/long", Topic: "pitfalls", Title: "the long form", Body: "A long body with the WHY and an example.", At: 1, Seq: 1, Kind: KindPitfalls})
	if rc := CmdKnowledge([]string{"supersede", "pitfalls/long", "--title", "short"}); rc != 0 {
		t.Fatal("supersede failed")
	}
	out := captureStdout(t, func() { CmdKnowledge([]string{"show", "pitfalls/long"}) })
	if !strings.Contains(out, "A long body with the WHY") {
		t.Fatalf("the superseded text must still be readable:\n%s", out)
	}
	old, fate, found := historyOf("pitfalls/long")
	if !found || old.Title != "the long form" || !strings.Contains(fate, "superseded by pitfalls/short") {
		t.Fatalf("history: %+v %q %v", old, fate, found)
	}
}

// ACE constraint: a render never rewrites the ledger.
func TestRenderNeverTouchesTheLedger(t *testing.T) {
	asHQ(t)
	writeOps(t, knowledgeOp{V: 2, Op: knowledgeOpAdd, ID: "pitfalls/x", Topic: "pitfalls", Title: "x", Body: "b", At: 1, Seq: 1, Kind: KindPitfalls})
	before, _ := os.ReadFile(knowledgeLedgerPath())
	if rc := CmdKnowledge([]string{"render"}); rc != 0 {
		t.Fatal("render")
	}
	after, _ := os.ReadFile(knowledgeLedgerPath())
	if string(before) != string(after) {
		t.Fatal("render rewrote the ledger")
	}
}
