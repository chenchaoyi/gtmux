package knowledge

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
)

// These origins and bodies are synthetic. Every writer runs under a disposable HOME.
func lineageOrigins(shape string) knowledgeOp {
	switch shape {
	case "range":
		return knowledgeOp{SeqRange: "110..125"}
	case "capture":
		return knowledgeOp{Seqs: []int64{140, 145}, Capture: "pitfalls/synthetic-capture"}
	default:
		return knowledgeOp{SeqRange: "110..125", Seqs: []int64{140, 145}, Capture: "pitfalls/synthetic-capture", Pane: "%900", Task: "synthetic-task", Legacy: true, Migration: "synthetic-archive-digest"}
	}
}

func lineageAssertOrigins(t *testing.T, got, want knowledgeOp) {
	t.Helper()
	if got.SeqRange != want.SeqRange || !reflect.DeepEqual(got.Seqs, want.Seqs) || got.Capture != want.Capture || got.Pane != want.Pane || got.Task != want.Task || got.Legacy != want.Legacy || got.Migration != want.Migration {
		t.Errorf("origin metadata lost: got range=%q seqs=%v capture=%q pane=%q task=%q legacy=%v migration=%q; want %+v", got.SeqRange, got.Seqs, got.Capture, got.Pane, got.Task, got.Legacy, got.Migration, want)
	}
}

func lineageLive(t *testing.T, id string) knowledgeOp {
	t.Helper()
	live, err := liveKnowledge()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := findLive(live, id)
	if !ok {
		t.Fatalf("missing live entry %s", id)
	}
	return e
}

func lineageSeed(t *testing.T, shape string) knowledgeOp {
	t.Helper()
	asHQ(t)
	e := lineageOrigins(shape)
	e.Op, e.ID, e.Topic = knowledgeOpAdd, "pitfalls/synthetic-source", "pitfalls"
	e.Title, e.Body, e.At, e.Seq = "Synthetic source", "Check the fixture before replacing it.", 10, 75
	e.Kind, e.Provenance, e.Tags, e.Hits, e.HitLast = KindPitfalls, ProvCorrection, []string{"synthetic-tag"}, 3, 9
	e.Lang = "en"
	e.Alt = &altHalf{Lang: "zh", Title: "旧译文", Body: "仅用于隔离测试。"}
	e.Sources = []Candidate{{ID: "synthetic-original", Digest: "synthetic-digest", At: 8, Topic: "pitfalls", Key: "pitfalls/synthetic-source", Lesson: "Preserve synthetic evidence.", Seq: 70, Pane: "%901", Task: "synthetic-origin-task", Source: "transcript", Context: "Synthetic context", Session: "synthetic-session", Project: "/synthetic/project", Count: 2, Agent: "synthetic-agent", Speaker: "user", ObservedAt: 7, SourceFile: "synthetic.jsonl", SourceOffset: 12, SourceTurn: "synthetic-turn"}}
	e.Memory = &MemoryRule{Actions: []string{"replace"}, Summary: "Check the fixture", Scope: "hq", Confidence: "supported", VerifiedAt: 5, Evidence: []string{"synthetic:evidence"}, ProtectedReason: "synthetic protection"}
	e.Sensitive, e.Confirmed = true, "Synthetic confirmation only."
	e.Status, e.Audience, e.PromotedAt, e.LandedAt, e.LandedRef = StatusHypothesis, AudienceHQ, 11, 12, "synthetic-old-carrier"
	e.PromoteWhy, e.PromoteTarget = "synthetic distribution", "synthetic target"
	if err := appendKnowledgeOp(e); err != nil {
		t.Fatal(err)
	}
	if err := commitKnowledgeOp(knowledgeOp{Op: knowledgeOpHit, ID: e.ID, Hits: 4, At: 20}, "synthetic later hit"); err != nil {
		t.Fatal(err)
	}
	return lineageLive(t, e.ID)
}

func lineageLedger(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(knowledgeLedgerPath())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func lineageShowAndRender(t *testing.T, e knowledgeOp) {
	t.Helper()
	output := captureStdout(t, func() {
		if rc := CmdKnowledge([]string{"show", e.ID, "--json"}); rc != 0 {
			t.Errorf("show exit %d", rc)
		}
	})
	var shown entryRead
	if err := json.Unmarshal([]byte(output), &shown); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(shown, readEntry(e)) {
		t.Fatal("latest show differs from live state")
	}
	b, err := os.ReadFile(topicPath(e.Topic))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{e.Title, e.Body, provenanceFooter(e)} {
		if !strings.Contains(string(b), want) {
			t.Errorf("render missing %q", want)
		}
	}
}

// KBL-001: the general mutation path, including a retitle to a new live ID.
func TestSourceLineageOrdinarySupersede(t *testing.T) {
	for _, shape := range []string{"range", "capture", "all"} {
		t.Run(shape, func(t *testing.T) {
			e := lineageSeed(t, shape)
			before, seq := lineageLedger(t), events.LatestSeq()
			if rc := CmdKnowledge([]string{"supersede", e.ID, "--title", "Synthetic revised source", "--why", "synthetic clarification"}); rc != 0 {
				t.Fatalf("supersede exit %d", rc)
			}
			ops, err := readKnowledgeOps()
			if err != nil {
				t.Fatal(err)
			}
			op := ops[len(ops)-1]
			lineageAssertOrigins(t, op, e)
			got := lineageLive(t, op.ID)
			lineageAssertOrigins(t, got, e)
			if got.Seq != seq || got.At <= e.At || op.OpID == e.OpID || got.Seq == e.Seq {
				t.Fatal("revision reused old event identity")
			}
			if got.Hits != 7 || got.HitLast != 20 || got.Provenance != e.Provenance || !reflect.DeepEqual(got.Tags, e.Tags) || !reflect.DeepEqual(got.Sources, e.Sources) {
				t.Fatal("ordinary revision lost latest observations or structured sources")
			}
			if !got.Sensitive || got.Confirmed != e.Confirmed || got.Memory.ProtectedReason != e.Memory.ProtectedReason || got.Memory.VerifiedAt != 0 {
				t.Fatal("ordinary revision lost protection or refreshed verification")
			}
			if got.Alt != nil || got.PromotedAt != 0 || got.Audience != "" || got.Status != "" {
				t.Fatal("ordinary revision changed translation/distribution reset contract")
			}
			if !bytes.HasPrefix(lineageLedger(t), before) {
				t.Fatal("rewrote old ledger")
			}
			live, _ := liveKnowledge()
			if _, ok := findLive(live, e.ID); ok {
				t.Fatal("retitled predecessor revived")
			}
			lineageShowAndRender(t, got)
		})
	}
}

func lineageFiles(t *testing.T) map[string][32]byte {
	t.Helper()
	files := map[string][32]byte{}
	err := filepath.WalkDir(os.Getenv("HOME"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[path] = sha256.Sum256(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// KBL-002: actual review check/apply, exact receipt and latest-state protection.
func TestSourceLineageReviewApply(t *testing.T) {
	for _, shape := range []string{"range", "capture", "all"} {
		t.Run(shape, func(t *testing.T) {
			e := lineageSeed(t, shape)
			guard := knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/synthetic-guard", Topic: "pitfalls", Title: "Synthetic guard", Body: "Keep concurrent observations.", Hits: 20}
			if err := appendKnowledgeOp(guard); err != nil {
				t.Fatal(err)
			}
			if err := commitKnowledgeOp(knowledgeOp{Op: knowledgeOpHit, ID: guard.ID, Hits: 3, At: 30}, "synthetic guard hit"); err != nil {
				t.Fatal(err)
			}
			guard = lineageLive(t, guard.ID)
			p := proposalFor(e)
			before, files := lineageLedger(t), lineageFiles(t)
			if r, err := ApplyReview(p, true); err != nil || r.Committed {
				t.Fatalf("check: %+v %v", r, err)
			}
			if !reflect.DeepEqual(files, lineageFiles(t)) {
				t.Fatal("dry run changed files")
			}
			ops, err := readKnowledgeOps()
			if err != nil {
				t.Fatal(err)
			}
			prepared, _, err := prepareReview(p, ops, 40)
			if err != nil {
				t.Fatal(err)
			}
			lineageAssertOrigins(t, prepared[0], e)
			if len(prepared[0].Seqs) > 0 {
				prepared[0].Seqs[0]++
				if lineageLive(t, e.ID).Seqs[0] != e.Seqs[0] {
					t.Fatal("preparation mutated persisted origins")
				}
			}
			seq := events.LatestSeq()
			if r, err := ApplyReview(p, false); err != nil || !r.Committed {
				t.Fatalf("apply: %+v %v", r, err)
			}
			ops, err = readKnowledgeOps()
			if err != nil {
				t.Fatal(err)
			}
			op := ops[len(ops)-1]
			lineageAssertOrigins(t, op, e)
			got := lineageLive(t, e.ID)
			lineageAssertOrigins(t, got, e)
			if got.Seq != seq || got.Seq == e.Seq || got.At <= e.At || got.OpID == e.OpID || got.OpID != "review-"+p.ID+"-0" {
				t.Fatal("review reused old operation identity")
			}
			if op.CandidateResult != "" || op.ReviewOrigin != p.Origin || !reflect.DeepEqual(op.ReviewEvidence, p.Changes[0].Evidence) {
				t.Fatal("review corrupted current operation evidence or receipt")
			}
			if got.Title != p.Changes[0].Title || got.Body != p.Changes[0].Body || entryRevision(got) == entryRevision(e) {
				t.Fatal("review failed to advance content/revision")
			}
			if got.Hits != 7 || got.HitLast != 20 || got.Provenance != e.Provenance || !reflect.DeepEqual(got.Tags, e.Tags) || !reflect.DeepEqual(got.Sources, e.Sources) {
				t.Fatal("review lost latest observations or structured sources")
			}
			if got.Lang != e.Lang || got.Alt != nil || got.Status != e.Status || got.Audience != e.Audience || got.LandedAt != 0 || got.LandedRef != "" || !promotionPending(got) {
				t.Fatal("review changed language/status/distribution contract")
			}
			if !got.Sensitive || got.Confirmed != e.Confirmed || got.Memory.ProtectedReason != e.Memory.ProtectedReason || got.Memory.Scope != e.Memory.Scope || got.Memory.VerifiedAt != 0 || got.Memory.Confidence != "tentative" {
				t.Fatal("review lost protection or refreshed verification")
			}
			if !bytes.HasPrefix(lineageLedger(t), before) {
				t.Fatal("rewrote old ledger")
			}
			lineageShowAndRender(t, got)
			after := lineageLedger(t)
			stale := proposalFor(e)
			stale.Changes[0].Body = "A different stale revision."
			stale.ID = proposalDigest(stale)
			if _, err := ApplyReview(stale, false); err == nil {
				t.Fatal("accepted stale proposal")
			}
			if r, err := ApplyReview(p, false); err != nil || !r.AlreadyApplied {
				t.Fatalf("retry: %+v %v", r, err)
			}
			if !bytes.Equal(after, lineageLedger(t)) {
				t.Fatal("conflict/retry appended")
			}
			if err := commitKnowledgeOp(knowledgeOp{Op: knowledgeOpHit, ID: e.ID, Hits: 2, At: got.At + 1}, "synthetic post-review hit"); err != nil {
				t.Fatal(err)
			}
			if r, err := ApplyReview(p, false); err != nil || !r.AlreadyApplied {
				t.Fatalf("later retry: %+v %v", r, err)
			}
			if latest := lineageLive(t, e.ID); latest.Hits != 9 || latest.HitLast != got.At+1 {
				t.Fatal("retry overwrote later hits")
			}
			if !reflect.DeepEqual(lineageLive(t, guard.ID), guard) {
				t.Fatal("overwrote unrelated updated guard")
			}
		})
	}
}

// KBL-003: source inheritance must not recover metadata from dead revisions.
func TestSourceLineageReplay(t *testing.T) {
	for _, reviewed := range []bool{false, true} {
		for _, explicit := range []bool{false, true} {
			t.Run(strings.Join([]string{map[bool]string{false: "ordinary", true: "reviewed"}[reviewed], map[bool]string{false: "omitted", true: "explicit"}[explicit]}, "/"), func(t *testing.T) {
				e := lineageOrigins("all")
				e.Op, e.ID, e.Topic, e.Title, e.Seq = knowledgeOpAdd, "pitfalls/synthetic", "pitfalls", "Synthetic", 60
				e.Sources = []Candidate{{ID: "old", Lesson: "Synthetic original", SourceFile: "synthetic-origin"}}
				op := knowledgeOp{Op: knowledgeOpSupersede, ID: e.ID, Supersedes: e.ID, Topic: e.Topic, Title: "Synthetic revised", Seq: 200, Reviewed: reviewed, Sources: []Candidate{e.Sources[0], {ID: "new", Lesson: "Synthetic new", SourceOffset: 42}}}
				want := e
				if explicit {
					op.SeqRange, op.Seqs, op.Capture, op.Pane, op.Task, op.Migration = "300..310", []int64{305}, "pitfalls/new", "%902", "new-task", "new-archive"
					want = op
					want.Legacy = true
				}
				input := []knowledgeOp{e, op}
				original, _ := json.Marshal(input)
				live := foldKnowledge(input)
				if len(live) != 1 {
					t.Fatal("wrong live entries")
				}
				got := live[0]
				lineageAssertOrigins(t, got, want)
				if got.Seq != 200 || got.CandidateResult != "" || len(got.Sources) != 2 || !reflect.DeepEqual(got.Sources[0], e.Sources[0]) || got.Sources[1].SourceOffset != 42 {
					t.Fatal("replay corrupted event, receipt or Sources")
				}
				if !explicit {
					got.Seqs[0]++
					after, _ := json.Marshal(input)
					if !bytes.Equal(original, after) {
						t.Fatal("inherited origins alias input ledger")
					}
				}
				retired := foldKnowledge([]knowledgeOp{e, {Op: knowledgeOpRetire, ID: e.ID}, op})
				if len(retired) != 1 || retired[0].Legacy || retired[0].Migration == e.Migration {
					t.Fatal("retired predecessor origins revived")
				}
				middle := knowledgeOp{Op: knowledgeOpSupersede, ID: "pitfalls/middle", Supersedes: e.ID, Topic: e.Topic, Title: "Synthetic middle"}
				latest := knowledgeOp{Op: knowledgeOpSupersede, ID: e.ID, Supersedes: middle.ID, Topic: e.Topic, Title: "Synthetic latest", SeqRange: "400..410", Seqs: []int64{405}, Capture: "pitfalls/latest", Pane: "%903", Task: "latest-task", Migration: "latest-archive"}
				successor := knowledgeOp{Op: knowledgeOpSupersede, ID: e.ID, Supersedes: e.ID, Topic: e.Topic, Title: "Synthetic final", Reviewed: reviewed}
				chain := foldKnowledge([]knowledgeOp{e, middle, latest, successor})
				latest.Legacy = true
				if len(chain) != 1 {
					t.Fatal("old revisions revived")
				}
				lineageAssertOrigins(t, chain[0], latest)
			})
		}
	}
}

// KBL-004: inherited origins never turn a revision into a repeated acceptance.
func TestSourceLineageNewCapture(t *testing.T) {
	e := lineageSeed(t, "all")
	c := Candidate{At: 35, Topic: "pitfalls", Key: "pitfalls/synthetic-new-capture", Lesson: "Keep this new synthetic observation.", Seq: 250, Pane: "%904", Task: "new-candidate-task", Count: 2}
	if err := AppendCandidate(c); err != nil {
		t.Fatal(err)
	}
	if rc := CmdKnowledge([]string{"supersede", e.ID, "--title", "Synthetic revised capture", "--capture", c.Key}); rc != 0 {
		t.Fatalf("supersede capture exit %d", rc)
	}
	ops, err := readKnowledgeOps()
	if err != nil {
		t.Fatal(err)
	}
	op := ops[len(ops)-1]
	got := lineageLive(t, op.ID)
	want := e
	want.Seqs = []int64{250}
	want.Capture = c.Key
	want.Pane = c.Pane
	want.Task = c.Task
	lineageAssertOrigins(t, op, want)
	lineageAssertOrigins(t, got, want)
	if len(got.Sources) != 2 || !reflect.DeepEqual(got.Sources[0], e.Sources[0]) || got.Sources[1].Key != c.Key || got.Hits != 9 {
		t.Fatal("new capture lost sources or counted hits incorrectly")
	}
	if op.CandidateResult != "accepted" || len(op.Sources) != 1 || op.Sources[0].Key != c.Key {
		t.Fatal("receipt includes sources not consumed by this operation")
	}
	pending, err := readCandidates()
	if err != nil || len(pending) != 0 {
		t.Fatalf("new capture not settled: %v %v", pending, err)
	}
	p := proposalFor(got)
	if _, err := ApplyReview(p, false); err != nil {
		t.Fatal(err)
	}
	ops, err = readKnowledgeOps()
	if err != nil {
		t.Fatal(err)
	}
	if ops[len(ops)-1].CandidateResult != "" || lineageLive(t, got.ID).Hits != 9 {
		t.Fatal("review repeated candidate acceptance")
	}
}
