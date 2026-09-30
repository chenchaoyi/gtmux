package knowledge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func sourceFixture() Candidate {
	return Candidate{ID: "source-1", At: 100, Topic: "pitfalls", Key: "pitfalls/shared", Lesson: "original correction",
		Context: "original assistant reply — private excerpt", Source: "transcript", Session: "session-1", Project: "repo-1",
		Pane: "%21", Task: "task-1", Seq: 15, Agent: "codex", Speaker: "user", ObservedAt: 80,
		SourceFile: "/synthetic/transcript.jsonl", SourceOffset: 123, SourceTurn: "turn-9"}
}

func addSource(t *testing.T, c Candidate) {
	t.Helper()
	if err := AppendCandidate(c); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureRejectedLedgerValidationKeepsPending(t *testing.T) {
	asHQ(t)
	c := sourceFixture()
	addSource(t, c)
	before, _ := os.ReadFile(pendingDistillPath())
	err := commitKnowledgeWithSources(knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/invalid", Topic: "pitfalls", Title: "invalid", Kind: "invented"}, []string{c.Key}, "add pitfalls/invalid")
	var result *CommitError
	if !errors.As(err, &result) || result.Committed || result.OpID == "" {
		t.Fatalf("failure must name an uncommitted operation: %v", err)
	}
	pending, err := readCandidates()
	if err != nil || len(pending) != 1 || pending[0].Context != c.Context {
		t.Fatalf("rejected write lost pending evidence: %+v, %v", pending, err)
	}
	after, _ := os.ReadFile(pendingDistillPath())
	if !bytes.Equal(before, after) {
		t.Fatal("a rejected write changed source bytes")
	}
	found := false
	for _, r := range events.Read(0, time.Now().Unix()+1) {
		if r.OpID == result.OpID {
			found = r.Outcome == "failed" && r.Phase == "ledger"
		}
	}
	if !found {
		t.Fatal("structured failure receipt missing")
	}
	log, err := os.ReadFile(diag.DayFile(state.LogsDir(), time.Now().Format("2006-01-02")))
	if err != nil || !bytes.Contains(log, []byte(result.OpID)) || bytes.Contains(log, []byte(c.Context)) {
		t.Fatalf("failure log must correlate without copying source text: %v", err)
	}
}

func TestCaptureLedgerWriteFailureKeepsPriorBytes(t *testing.T) {
	asHQ(t)
	if err := commitKnowledgeOp(knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/prior", Topic: "pitfalls", Title: "prior"}, "add pitfalls/prior"); err != nil {
		t.Fatal(err)
	}
	addSource(t, sourceFixture())
	prior, _ := os.ReadFile(knowledgeLedgerPath())
	if err := os.Chmod(knowledgeLedgerPath(), 0o400); err != nil {
		t.Fatal(err)
	}
	err := knowledgeAdd([]string{"--topic", "pitfalls", "--title", "write failed", "--capture", "pitfalls/shared"})
	var result *CommitError
	if !errors.As(err, &result) || result.Committed || result.Phase != "ledger" {
		t.Fatalf("wrong failure: %v", err)
	}
	after, _ := os.ReadFile(knowledgeLedgerPath())
	if !bytes.Equal(prior, after) || PendingCandidateCount() != 1 {
		t.Fatal("failed write changed ledger or consumed pending work")
	}
	if err := os.Chmod(knowledgeLedgerPath(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := knowledgeAdd([]string{"--topic", "pitfalls", "--title", "write recovered", "--capture", "pitfalls/shared"}); err != nil {
		t.Fatal(err)
	}
	if PendingCandidateCount() != 0 {
		t.Fatal("recovery did not settle source")
	}
}

func TestCaptureSourcesSurviveDetailHistoryAndRemainPrivate(t *testing.T) {
	asHQ(t)
	c := sourceFixture()
	addSource(t, c)
	if err := knowledgeAdd([]string{"--topic", "pitfalls", "--title", "source backed", "--capture", c.Key}); err != nil {
		t.Fatal(err)
	}
	op, _ := liveKnowledge()
	if len(op) != 1 || len(op[0].Sources) != 1 {
		t.Fatalf("source snapshot missing: %+v", op)
	}
	want := c
	want.Digest = candidateDigest(c)
	if !reflect.DeepEqual(want, op[0].Sources[0]) {
		t.Fatalf("source altered: %+v", op[0].Sources[0])
	}
	detail, ok := KnowledgeEntry(op[0].ID)
	if !ok || !reflect.DeepEqual(detail.Sources, op[0].Sources) {
		t.Fatal("owner detail lost sources")
	}
	show := captureStdout(t, func() {
		if rc := knowledgeShow([]string{op[0].ID, "--json"}); rc != 0 {
			t.Fatal(rc)
		}
	})
	if !strings.Contains(show, c.Context) {
		t.Fatal("show --json must expose evidence")
	}
	index, _ := KnowledgeIndexJSON(time.Now().Unix())
	render, _ := os.ReadFile(topicPath("pitfalls"))
	if bytes.Contains(index, []byte(c.Context)) || bytes.Contains(render, []byte(c.Context)) {
		t.Fatal("source text leaked into index/render")
	}
	if err := knowledgeSupersede([]string{op[0].ID, "--title", "source refined"}); err != nil {
		t.Fatal(err)
	}
	newOp, _ := liveKnowledge()
	if !reflect.DeepEqual(op[0].Sources, newOp[0].Sources) {
		t.Fatal("supersede lost source lineage")
	}
	if err := KnowledgeRetire(newOp[0].ID, "obsolete"); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if rc := knowledgeReceipts([]string{"--capture", c.Key, "--json"}); rc != 0 {
			t.Fatal(rc)
		}
	})
	var receipts []CandidateReceipt
	if err := json.Unmarshal([]byte(out), &receipts); err != nil || len(receipts) != 1 || receipts[0].Outcome != "accepted" || receipts[0].Sources[0].Digest != want.Digest {
		t.Fatalf("history receipt: %s, %v", out, err)
	}
	// Distribution sees the curated entry alone, not the source's private context.
	newOp[0].Audience = AudienceMachine
	if text := renderMachine([]knowledgeOp{newOp[0]}); strings.Contains(text, c.Context) {
		t.Fatal("source leaked into machine distribution")
	}
}

func TestCaptureLegacySourcesRetryAndNewFamilyObservation(t *testing.T) {
	asHQ(t)
	c := sourceFixture()
	c.ID = ""
	b, _ := json.Marshal(c)
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	original := append(append(append([]byte{}, b...), '\n'), append(b, '\n')...)
	if err := os.WriteFile(pendingDistillPath(), original, 0o600); err != nil {
		t.Fatal(err)
	}
	first, _ := readCandidates()
	second, _ := readCandidates()
	if len(first) != 2 || first[0].ID == first[1].ID || !reflect.DeepEqual(first, second) {
		t.Fatal("legacy identities are unstable or collapse observations")
	}
	if err := knowledgeAdd([]string{"--topic", "pitfalls", "--title", "legacy accepted", "--capture", c.Key}); err != nil {
		t.Fatal(err)
	}
	if PendingCandidateCount() != 0 {
		t.Fatal("legacy sources still pending")
	}
	if err := knowledgeAdd([]string{"--topic", "pitfalls", "--title", "duplicate attempt", "--capture", c.Key}); err == nil {
		t.Fatal("same sources were accepted twice")
	}
	ops, _ := readKnowledgeOps()
	if len(ops) != 1 || len(ops[0].Sources) != 2 {
		t.Fatal("retry changed source history")
	}
	after, _ := os.ReadFile(pendingDistillPath())
	if !bytes.Equal(original, after) {
		t.Fatal("legacy file was rewritten")
	}
	c.ID = "new-observation"
	addSource(t, c)
	pending, _ := readCandidates()
	if len(pending) != 1 || pending[0].ID != c.ID {
		t.Fatalf("family tombstone swallowed new observation: %+v", pending)
	}
}

func TestCaptureDismissalRetainsReasonAndUnknownBatchIsAtomic(t *testing.T) {
	asHQ(t)
	c := sourceFixture()
	addSource(t, c)
	if err := knowledgeDismiss([]string{"--capture", c.Key + ",pitfalls/missing", "--why", "not durable"}); err == nil || PendingCandidateCount() != 1 {
		t.Fatal("unknown batch consumed a source")
	}
	if err := knowledgeDismiss([]string{"--capture", c.Key, "--why", strings.Repeat("x", 301)}); err == nil || PendingCandidateCount() != 1 {
		t.Fatal("invalid reason consumed a source")
	}
	if err := knowledgeDismiss([]string{"--capture", c.Key, "--why", "same episode, no reusable lesson"}); err != nil {
		t.Fatal(err)
	}
	ops, _ := readKnowledgeOps()
	if len(ops) != 1 || ops[0].CandidateResult != "dismissed" || ops[0].Why != "same episode, no reusable lesson" || len(ops[0].Sources) != 1 || len(foldKnowledge(ops)) != 0 {
		t.Fatalf("dismissal evidence: %+v", ops)
	}
	// Resubmitting the mined source cannot resurrect it or append an extra observation.
	addSource(t, c)
	all, _ := readCandidateSources()
	if len(all) != 1 || PendingCandidateCount() != 0 {
		t.Fatal("duplicate source append is not idempotent")
	}
	c.Context = "different content"
	if err := AppendCandidate(c); err == nil {
		t.Fatal("identity with changed evidence must refuse")
	}
}

func TestCaptureCommittedRenderFailureHasHonestReceipt(t *testing.T) {
	asHQ(t)
	addSource(t, sourceFixture())
	if err := os.MkdirAll(MachinePath(), 0o755); err != nil {
		t.Fatal(err)
	}
	err := knowledgeAdd([]string{"--topic", "pitfalls", "--title", "view blocked", "--capture", "pitfalls/shared"})
	var result *CommitError
	if !errors.As(err, &result) || !result.Committed || result.Phase != "render" || !strings.Contains(err.Error(), "render") {
		t.Fatalf("failure must identify committed work and repair: %v", err)
	}
	if PendingCandidateCount() != 0 {
		t.Fatal("committed sources returned to pending")
	}
	if err := knowledgeAdd([]string{"--topic", "pitfalls", "--title", "retry view blocked", "--capture", "pitfalls/shared"}); err == nil {
		t.Fatal("render failure allowed double settlement")
	}
	ops, _ := readKnowledgeOps()
	if len(ops) != 1 || ops[0].OpID != result.OpID {
		t.Fatal("committed receipt lost")
	}
	found := false
	for _, r := range events.Read(0, time.Now().Unix()+1) {
		if r.OpID == result.OpID && r.Outcome == "committed-view-failed" && r.Phase == "render" {
			found = true
		}
	}
	if !found {
		t.Fatal("post-commit failure receipt missing")
	}
	if err := os.Remove(MachinePath()); err != nil {
		t.Fatal(err)
	}
	if err := knowledgeRender(nil); err != nil {
		t.Fatal(err)
	}
}

func TestCandidateProcessHelper(t *testing.T) {
	action := os.Getenv("GTMUX_KB_TEST_ACTION")
	if action == "" {
		return
	}
	if action == "accept" {
		if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", os.Getenv("GTMUX_KB_TEST_ID"), "--capture", "pitfalls/shared"}); rc != 0 {
			os.Exit(3)
		}
	} else {
		id := os.Getenv("GTMUX_KB_TEST_ID")
		if err := AppendCandidate(Candidate{ID: id, Topic: "pitfalls", Key: "pitfalls/" + id, Lesson: id}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCaptureCrossProcessSettlementAndAppends(t *testing.T) {
	asHQ(t)
	addSource(t, sourceFixture())
	var cmds []*exec.Cmd
	for i := 0; i < 8; i++ {
		action := "append"
		if i < 2 {
			action = "accept"
		}
		cmd := exec.Command(os.Args[0], "-test.run=^TestCandidateProcessHelper$")
		cmd.Env = append(os.Environ(), "GTMUX_KB_TEST_ACTION="+action, fmt.Sprintf("GTMUX_KB_TEST_ID=process-%d", i))
		cmd.Dir = state.HQHome()
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, cmd)
	}
	accepted := 0
	for i, cmd := range cmds {
		err := cmd.Wait()
		if i < 2 {
			if err == nil {
				accepted++
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if accepted != 1 {
		t.Fatalf("want one settlement winner, got %d", accepted)
	}
	ops, _ := readKnowledgeOps()
	pending, _ := readCandidates()
	all, _ := readCandidateSources()
	if len(ops) != 1 || len(ops[0].Sources) != 1 || len(pending) != 6 || len(all) != 7 {
		t.Fatalf("cross-process writes lost or double-settled work: ops=%d pending=%d sources=%d", len(ops), len(pending), len(all))
	}
	leftovers, _ := filepath.Glob(filepath.Join(Dir(), ".knowledge-write-*"))
	if len(leftovers) != 0 {
		t.Fatalf("temporary writes leaked: %v", leftovers)
	}
}

func TestAtomicAppendFailureAndPartialLegacyTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger")
	if err := os.WriteFile(path, []byte("legacy-tail"), 0o600); err != nil {
		t.Fatal(err)
	}
	if committed, err := atomicAppend(path, []byte("next\n")); err != nil || !committed {
		t.Fatalf("append: %v", err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "legacy-tail\nnext\n" {
		t.Fatalf("tail swallowed the next record: %q", b)
	}
	if committed, err := atomicAppend(path, bytes.Repeat([]byte("x"), 1024*1024)); err == nil || committed {
		t.Fatal("oversized append did not refuse")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(b, after) {
		t.Fatal("failed append changed history")
	}
}

func TestCaptureDigestMismatchRefusesSettlement(t *testing.T) {
	asHQ(t)
	addSource(t, sourceFixture())
	b, _ := os.ReadFile(pendingDistillPath())
	b = bytes.Replace(b, []byte("original correction"), []byte("changed correction"), 1)
	if err := os.WriteFile(pendingDistillPath(), b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := knowledgeAdd([]string{"--topic", "pitfalls", "--title", "tampered", "--capture", "pitfalls/shared"}); err == nil {
		t.Fatal("changed evidence was silently accepted")
	}
	if PendingCandidateCount() != 1 {
		t.Fatal("changed evidence was consumed")
	}
	ops, _ := readKnowledgeOps()
	if len(ops) != 0 {
		t.Fatal("refused source wrote a settlement")
	}
}

func TestCapturedSupersedeKeepsAllSourcesAndRecurrence(t *testing.T) {
	asHQ(t)
	c := sourceFixture()
	addSource(t, c)
	if err := knowledgeAdd([]string{"--topic", "pitfalls", "--title", "first", "--capture", c.Key}); err != nil {
		t.Fatal(err)
	}
	c.ID = "source-2"
	c.Count = 2
	c.Lesson = "second episode"
	addSource(t, c)
	if err := knowledgeSupersede([]string{"pitfalls/first", "--title", "revised", "--capture", c.Key}); err != nil {
		t.Fatal(err)
	}
	live, _ := liveKnowledge()
	if len(live) != 1 || len(live[0].Sources) != 2 || live[0].Hits != 3 || PendingCandidateCount() != 0 {
		t.Fatalf("lost lineage/count: %+v", live)
	}
	if id, ok, err := HitByCaptureKey(c.Key, 1, 200); err != nil || !ok || id != "pitfalls/revised" {
		t.Fatalf("recurrence missed successor: %s %v %v", id, ok, err)
	}
}

func TestSourceFilesStayPrivateAndDoNotCreateV1Backup(t *testing.T) {
	asHQ(t)
	addSource(t, sourceFixture())
	for _, title := range []string{"first", "second"} {
		if err := knowledgeAdd([]string{"--topic", "pitfalls", "--title", title}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(knowledgeLedgerPath() + ".bak-v1"); !os.IsNotExist(err) {
		t.Fatal("v3 history was misclassified as v1 and copied")
	}
	for _, path := range []string{knowledgeLedgerPath(), pendingDistillPath()} {
		st, err := os.Stat(path)
		if err != nil || st.Mode().Perm() != 0600 {
			t.Fatalf("source file is not private: %s %v", path, err)
		}
	}
}
