package hq

import (
	"os"
	"strings"
	"testing"
	"time"

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

// The export lifecycle end to end: promote writes the brief, promotions lists it,
// land removes it and closes the loop — with every refusal on the way.
func TestKnowledgePromotionVerbChain(t *testing.T) {
	asHQ(t)
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls",
		"--title", "goal text must ride a file, never argv"}); rc != 0 {
		t.Fatal("add failed")
	}
	id := "pitfalls/" + slug("goal text must ride a file, never argv")

	// promote needs --why; land refuses before any promotion exists.
	if rc := CmdKnowledge([]string{"promote", id}); rc == 0 {
		t.Fatal("promote without --why must refuse")
	}
	if rc := CmdKnowledge([]string{"land", id, "--ref", "#812"}); rc == 0 {
		t.Fatal("landing a never-promoted entry must refuse")
	}

	if rc := CmdKnowledge([]string{"promote", id,
		"--why", "holds on any machine and belongs in the dispatch spec",
		"--target", "agent-dispatch spec"}); rc != 0 {
		t.Fatal("promote failed")
	}
	// The brief exists and carries the case + provenance + closing instruction.
	brief, err := os.ReadFile(promotionBriefPath(knowledgeOp{ID: id}))
	if err != nil {
		t.Fatalf("brief missing: %v", err)
	}
	for _, want := range []string{"goal text must ride a file", "belongs in the dispatch spec",
		"agent-dispatch spec", "gtmux knowledge land " + id} {
		if !strings.Contains(string(brief), want) {
			t.Errorf("brief missing %q:\n%s", want, brief)
		}
	}
	// The topic render badges the pending state.
	topic, _ := os.ReadFile(topicPath("pitfalls"))
	if !strings.Contains(string(topic), "⚑ promoted (pending)") {
		t.Errorf("topic render must badge a pending promotion:\n%s", topic)
	}
	// A second promote while pending refuses.
	if rc := CmdKnowledge([]string{"promote", id, "--why", "again"}); rc == 0 {
		t.Fatal("promoting an already-pending entry must refuse")
	}

	if rc := CmdKnowledge([]string{"land", id}); rc == 0 {
		t.Fatal("land without --ref must refuse")
	}
	if rc := CmdKnowledge([]string{"land", id, "--ref", "gtmux#812"}); rc != 0 {
		t.Fatal("land failed")
	}
	// The brief is gone, the badge flips, the queue is clear.
	if _, err := os.Stat(promotionBriefPath(knowledgeOp{ID: id})); !os.IsNotExist(err) {
		t.Fatal("a landed promotion's brief must be removed")
	}
	topic, _ = os.ReadFile(topicPath("pitfalls"))
	if !strings.Contains(string(topic), "→ landed gtmux#812") ||
		strings.Contains(string(topic), "⚑") {
		t.Errorf("topic render must badge the landed state:\n%s", topic)
	}
	if rc := CmdKnowledge([]string{"land", id, "--ref", "again"}); rc == 0 {
		t.Fatal("landing twice must refuse — nothing is pending")
	}

	audits := knowledgeAudits(t)
	joined := strings.Join(audits, " | ")
	if !strings.Contains(joined, "promote "+id) || !strings.Contains(joined, "land "+id+" → gtmux#812") {
		t.Fatalf("both lifecycle ops must be journaled: %v", audits)
	}
}

// The doctor verdict over the queue: empty quiet, young OK, stale flagged,
// landed back to quiet.
func TestPromotionsStatus(t *testing.T) {
	asHQ(t)
	now := int64(2_000_000_000)
	if r := PromotionsStatus(now); r.Pending != 0 || r.State != MaintenanceOK {
		t.Fatalf("empty queue must be quiet: %+v", r)
	}
	if rc := CmdKnowledge([]string{"add", "--topic", "workflows", "--title", "release flow"}); rc != 0 {
		t.Fatal("add failed")
	}
	id := "workflows/" + slug("release flow")
	if rc := CmdKnowledge([]string{"promote", id, "--why", "w"}); rc != 0 {
		t.Fatal("promote failed")
	}
	promotedAt := time.Now().Unix()
	if r := PromotionsStatus(promotedAt + 3600); r.Pending != 1 || r.State != MaintenanceOK {
		t.Fatalf("a young pending promotion is OK with figures: %+v", r)
	}
	if r := PromotionsStatus(promotedAt + promotionStaleSecs + 1); r.State != MaintenanceSlipped {
		t.Fatalf("a stale pending promotion must flag: %+v", r)
	}
	if rc := CmdKnowledge([]string{"land", id, "--ref", "#1"}); rc != 0 {
		t.Fatal("land failed")
	}
	if r := PromotionsStatus(promotedAt + promotionStaleSecs + 2); r.Pending != 0 || r.State != MaintenanceOK {
		t.Fatalf("landing must clear the row: %+v", r)
	}
}

// A pending brief is regenerated by every mutation's render pass — deleting it by
// hand cannot lose the hand-off while the promotion is still open.
func TestPromotionBriefRegeneratesOnUnrelatedMutation(t *testing.T) {
	asHQ(t)
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "charter lesson"}); rc != 0 {
		t.Fatal("add failed")
	}
	id := "pitfalls/" + slug("charter lesson")
	if rc := CmdKnowledge([]string{"promote", id, "--why", "w"}); rc != 0 {
		t.Fatal("promote failed")
	}
	brief := promotionBriefPath(knowledgeOp{ID: id})
	if err := os.Remove(brief); err != nil {
		t.Fatal(err)
	}
	// An UNRELATED mutation re-renders the outbox and the brief comes back.
	if rc := CmdKnowledge([]string{"add", "--topic", "workflows", "--title", "unrelated"}); rc != 0 {
		t.Fatal("unrelated add failed")
	}
	if _, err := os.Stat(brief); err != nil {
		t.Fatalf("the pending brief must regenerate on the next render pass: %v", err)
	}
}
