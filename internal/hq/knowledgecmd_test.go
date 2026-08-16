package hq

import (
	"os"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
)

func knowledgeAudits(t *testing.T) []string {
	t.Helper()
	recs, _ := events.ReadSince(0)
	var out []string
	for _, r := range recs {
		if r.Event == events.AuditEventKnowledge {
			out = append(out, r.Summary)
		}
	}
	return out
}

// The role gate: a worker's directory cannot write knowledge; the read side is open.
func TestKnowledgeMutationsRefusedOutsideHQHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "x"}); rc == 0 {
		t.Fatal("add outside the HQ home must refuse")
	}
	if _, err := os.Stat(knowledgeLedgerPath()); !os.IsNotExist(err) {
		t.Fatal("a refused mutation must write nothing")
	}
	if rc := CmdKnowledge([]string{"list"}); rc != 0 {
		t.Fatal("list is read-only and open to anyone")
	}
}

// The full verb chain: add → supersede → retire, each journaled, the render kept current.
func TestKnowledgeVerbChain(t *testing.T) {
	asHQ(t)
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls",
		"--title", "wrangler TLS-resets from the office"}); rc != 0 {
		t.Fatal("add failed")
	}
	// The render exists, is marked, and carries the entry.
	b, err := os.ReadFile(topicPath("pitfalls"))
	if err != nil || !strings.HasPrefix(string(b), knowledgeRenderMarker) ||
		!strings.Contains(string(b), "wrangler TLS-resets") {
		t.Fatalf("render missing or wrong after add: %v\n%s", err, b)
	}
	// A same-title re-add collides loudly and appends nothing.
	before, _ := readKnowledgeOps()
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls",
		"--title", "wrangler TLS-resets from the office"}); rc == 0 {
		t.Fatal("an id collision must refuse")
	}
	if after, _ := readKnowledgeOps(); len(after) != len(before) {
		t.Fatal("a refused add appended to the ledger")
	}

	id := "pitfalls/" + slug("wrangler TLS-resets from the office")
	if rc := CmdKnowledge([]string{"supersede", id, "--title", "office TLS resets: always retry wrangler"}); rc != 0 {
		t.Fatal("supersede failed")
	}
	newID := "pitfalls/" + slug("office TLS resets: always retry wrangler")
	live, _ := liveKnowledge()
	if _, ok := findLive(live, id); ok {
		t.Fatal("the superseded predecessor must be dead")
	}
	if _, ok := findLive(live, newID); !ok {
		t.Fatal("the successor must be live")
	}

	// retire needs --why; with it, the entry dies and the render empties.
	if rc := CmdKnowledge([]string{"retire", newID}); rc == 0 {
		t.Fatal("retire without --why must refuse")
	}
	if rc := CmdKnowledge([]string{"retire", newID, "--why", "the office network was fixed"}); rc != 0 {
		t.Fatal("retire failed")
	}
	b, _ = os.ReadFile(topicPath("pitfalls"))
	if strings.Contains(string(b), "TLS") {
		t.Fatalf("a retired entry must leave the render:\n%s", b)
	}

	audits := knowledgeAudits(t)
	if len(audits) != 3 {
		t.Fatalf("three mutations, %d audit records: %v", len(audits), audits)
	}
	for i, want := range []string{"add " + id, "supersede " + id, "retire " + newID} {
		if !strings.Contains(audits[i], want) {
			t.Errorf("audit[%d] = %q, want ~%q", i, audits[i], want)
		}
	}
}

// --capture consumes every same-key candidate into ONE entry that inherits their
// provenance; unrelated candidates stay pending.
func TestKnowledgeAddConsumesCaptureWithProvenance(t *testing.T) {
	asHQ(t)
	key := "pitfalls/" + slug("wrangler needs a retry")
	for i, c := range []captureCandidate{
		{At: 100, Topic: "pitfalls", Key: key, Lesson: "wrangler needs a retry", Pane: "%21", Seq: 6650},
		{At: 200, Topic: "pitfalls", Key: key, Lesson: "wrangler needs a retry (again)", Pane: "%30", Seq: 6800, Task: "t-9"},
		{At: 300, Topic: "workflows", Key: "workflows/other", Lesson: "unrelated", Seq: 6900},
	} {
		if err := appendCandidate(c); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls",
		"--title", "wrangler needs a retry", "--capture", key}); rc != 0 {
		t.Fatal("add --capture failed")
	}
	live, _ := liveKnowledge()
	entry, ok := findLive(live, "pitfalls/"+slug("wrangler needs a retry"))
	if !ok {
		t.Fatal("entry missing")
	}
	if len(entry.Seqs) != 2 || entry.Seqs[0] != 6650 || entry.Seqs[1] != 6800 {
		t.Errorf("both candidates' seqs must survive: %v", entry.Seqs)
	}
	if entry.Pane != "%30" || entry.Task != "t-9" || entry.Capture != key {
		t.Errorf("newest candidate's pane/task and the key must survive: %+v", entry)
	}
	remaining, _ := readCandidates()
	if len(remaining) != 1 || remaining[0].Key != "workflows/other" {
		t.Fatalf("unrelated candidates must stay pending: %+v", remaining)
	}
	// The render's provenance footer shows the evidence.
	b, _ := os.ReadFile(topicPath("pitfalls"))
	if !strings.Contains(string(b), "seq 6650,6800") || !strings.Contains(string(b), "capture "+key) {
		t.Fatalf("provenance must reach the render:\n%s", b)
	}
}

// dismiss is the quality gate's rejection WITH a trace: spool line gone, no ledger
// op, one journal record carrying the reason.
func TestKnowledgeDismissLeavesATrace(t *testing.T) {
	asHQ(t)
	key := "pitfalls/noise"
	if err := appendCandidate(captureCandidate{At: 100, Topic: "pitfalls", Key: key,
		Lesson: "noise", Seq: 10}); err != nil {
		t.Fatal(err)
	}
	if rc := CmdKnowledge([]string{"dismiss", "--capture", key, "--why", "one-off, not cross-cutting"}); rc != 0 {
		t.Fatal("dismiss failed")
	}
	if n := pendingCandidateCount(); n != 0 {
		t.Fatalf("candidate still pending: %d", n)
	}
	if ops, _ := readKnowledgeOps(); len(ops) != 0 {
		t.Fatal("a dismissal is not a ledger operation")
	}
	audits := knowledgeAudits(t)
	if len(audits) != 1 || !strings.Contains(audits[0], "dismiss "+key) ||
		!strings.Contains(audits[0], "one-off") {
		t.Fatalf("the rejection must be journaled with its reason: %v", audits)
	}
	// An unknown key errors rather than silently succeeding.
	if rc := CmdKnowledge([]string{"dismiss", "--capture", "pitfalls/nope", "--why", "x"}); rc == 0 {
		t.Fatal("dismissing an unknown key must refuse")
	}
}

// render --check is the drift gate's CLI face.
func TestKnowledgeRenderCheckCLI(t *testing.T) {
	asHQ(t)
	if rc := CmdKnowledge([]string{"add", "--topic", "workflows", "--title", "release flow"}); rc != 0 {
		t.Fatal("add failed")
	}
	if rc := CmdKnowledge([]string{"render", "--check"}); rc != 0 {
		t.Fatal("a clean tree must pass --check")
	}
	f, _ := os.OpenFile(topicPath("workflows"), os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = f.WriteString("- hand edit\n")
	_ = f.Close()
	if rc := CmdKnowledge([]string{"render", "--check"}); rc == 0 {
		t.Fatal("--check must fail on a hand-edited render")
	}
	if rc := CmdKnowledge([]string{"render"}); rc != 0 {
		t.Fatal("render must restore")
	}
	if rc := CmdKnowledge([]string{"render", "--check"}); rc != 0 {
		t.Fatal("restored tree must pass again")
	}
}
