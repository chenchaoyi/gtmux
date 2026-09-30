package knowledge

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func migrationData(t *testing.T, ops ...knowledgeOp) []byte {
	t.Helper()
	var out bytes.Buffer
	for _, op := range ops {
		if op.V == 0 {
			op.V = 3
		}
		b, err := json.Marshal(op)
		if err != nil {
			t.Fatal(err)
		}
		out.Write(b)
		out.WriteByte('\n')
	}
	return out.Bytes()
}
func migrationPublic(t *testing.T) []byte {
	c := sourceFixture()
	c.Digest = candidateDigest(c)
	return migrationData(t,
		knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/old", Topic: "pitfalls", Title: "old lesson", Body: "old body", At: 1, Sources: []Candidate{c}},
		knowledgeOp{Op: knowledgeOpSupersede, ID: "pitfalls/new", Supersedes: "pitfalls/old", Topic: "pitfalls", Title: "new lesson", Body: "reviewed body", At: 2},
		knowledgeOp{Op: knowledgeOpPromote, ID: "pitfalls/new", Audience: AudienceMachine, Why: "old machine", At: 3},
		knowledgeOp{Op: knowledgeOpLand, ID: "pitfalls/new", Ref: "old machine.md", At: 4},
	)
}

func TestMigrationPreservesHistoryAndResetsAudience(t *testing.T) {
	asHQ(t)
	data := migrationPublic(t)
	preview, n, err := MigrationEntries(data, false)
	if err != nil || n != 0 || len(preview) != 1 || preview[0].Status != "new" || preview[0].Records != 4 {
		t.Fatalf("preview: %+v %d %v", preview, n, err)
	}
	if _, err := os.Stat(knowledgeLedgerPath()); !os.IsNotExist(err) {
		t.Fatal("preview wrote destination")
	}
	result, err := ApplyMigration(data, []string{"pitfalls/new"}, strings.Repeat("a", 64))
	if err != nil || !result.Committed || result.Imported != 1 {
		t.Fatalf("apply: %+v %v", result, err)
	}
	live, _ := liveKnowledge()
	if len(live) != 1 || len(live[0].Sources) != 1 || live[0].Sources[0].Context != sourceFixture().Context || promotionPending(live[0]) || live[0].Audience == AudienceMachine || live[0].LandedAt != 0 {
		t.Fatalf("lost evidence or inherited audience: %+v", live)
	}
	old, _, ok := historyOf("pitfalls/old")
	if !ok || old.Body != "old body" {
		t.Fatal("original revision history missing")
	}
	machine, _ := os.ReadFile(MachinePath())
	if bytes.Contains(machine, []byte("reviewed body")) {
		t.Fatal("imported knowledge distributed before a new audience decision")
	}
	before, _ := os.ReadFile(knowledgeLedgerPath())
	result, err = ApplyMigration(data, []string{"pitfalls/new"}, strings.Repeat("a", 64))
	after, _ := os.ReadFile(knowledgeLedgerPath())
	if err != nil || result.Imported != 0 || result.Skipped != 1 || !bytes.Equal(before, after) {
		t.Fatal("repeat migration changed history")
	}
}

func TestMigrationSensitiveHistoryAndUnrelatedEntriesExcluded(t *testing.T) {
	asHQ(t)
	data := migrationData(t,
		knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/public", Topic: "pitfalls", Title: "public"},
		knowledgeOp{Op: knowledgeOpAdd, ID: "accounts/secret", Topic: "accounts", Title: "PRIVATE", Sensitive: true, Confirmed: "explicit consent"},
		knowledgeOp{Op: knowledgeOpSensitive, ID: "accounts/secret", Sensitive: false, Confirmed: "now public"},
		knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/retired", Topic: "pitfalls", Title: "RETIRED"},
		knowledgeOp{Op: knowledgeOpRetire, ID: "pitfalls/retired", Why: "obsolete"},
		knowledgeOp{Op: knowledgeOpDismiss, ID: "dismiss/source", Why: "one-off", CandidateResult: "dismissed"},
	)
	preview, n, err := MigrationEntries(data, false)
	if err != nil || n != 1 || len(preview) != 1 || preview[0].ID != "pitfalls/public" {
		t.Fatalf("sensitive preview: %+v %d %v", preview, n, err)
	}
	selected, err := MigrationSelection(data, false)
	if err != nil || bytes.Contains(selected, []byte("PRIVATE")) || bytes.Contains(selected, []byte("RETIRED")) || bytes.Contains(selected, []byte("dismiss/source")) {
		t.Fatalf("unselected history leaked: %s %v", selected, err)
	}
	selected, err = MigrationSelection(data, true)
	if err != nil || !bytes.Contains(selected, []byte("PRIVATE")) {
		t.Fatal("explicit sensitive selection omitted")
	}
}

func TestMigrationConflictAndInvalidInputKeepPriorBytes(t *testing.T) {
	asHQ(t)
	if err := knowledgeAdd([]string{"--topic", "pitfalls", "--title", "existing"}); err != nil {
		t.Fatal(err)
	}
	prior, _ := os.ReadFile(knowledgeLedgerPath())
	for _, data := range [][]byte{
		migrationData(t, knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/new", Topic: "pitfalls", Title: "new"}, knowledgeOp{Op: knowledgeOpAdd, ID: "pitfalls/existing", Topic: "pitfalls", Title: "conflicting"}),
		[]byte("{bad json}\n"),
		migrationData(t, knowledgeOp{V: 99, Op: knowledgeOpAdd, ID: "pitfalls/new", Topic: "pitfalls", Title: "future"}),
		migrationData(t, knowledgeOp{Op: "unknown", ID: "pitfalls/new"}),
		migrationData(t, knowledgeOp{Op: knowledgeOpSupersede, ID: "pitfalls/new", Supersedes: "pitfalls/missing", Topic: "pitfalls", Title: "missing lineage"}),
	} {
		result, err := ApplyMigration(data, []string{"pitfalls/new", "pitfalls/existing"}, "archive")
		after, _ := os.ReadFile(knowledgeLedgerPath())
		if err == nil || result.Committed || !bytes.Equal(prior, after) {
			t.Fatalf("invalid/conflicting input modified destination: %+v %v", result, err)
		}
	}
}

func TestMigrationFailedWriteAndCommittedRenderFailure(t *testing.T) {
	asHQ(t)
	if err := knowledgeAdd([]string{"--topic", "pitfalls", "--title", "existing"}); err != nil {
		t.Fatal(err)
	}
	data := migrationPublic(t)
	prior, _ := os.ReadFile(knowledgeLedgerPath())
	if err := os.Chmod(knowledgeLedgerPath(), 0400); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyMigration(data, []string{"pitfalls/new"}, "archive")
	after, _ := os.ReadFile(knowledgeLedgerPath())
	if err == nil || result.Committed || !bytes.Equal(prior, after) {
		t.Fatal("failed write changed ledger")
	}
	if err := os.Chmod(knowledgeLedgerPath(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(MachinePath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(MachinePath(), 0700); err != nil {
		t.Fatal(err)
	}
	result, err = ApplyMigration(data, []string{"pitfalls/new"}, "archive")
	var failure *CommitError
	if !errors.As(err, &failure) || !failure.Committed || !result.Committed || result.Imported != 1 || result.Backup == "" {
		t.Fatalf("dishonest render failure: %+v %v", result, err)
	}
	backup, _ := os.ReadFile(result.Backup)
	if !bytes.Equal(prior, backup) {
		t.Fatal("pre-merge backup not exact")
	}
	if err := os.Remove(MachinePath()); err != nil {
		t.Fatal(err)
	}
	if err := knowledgeRender(nil); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationRejectsUnknownFieldsInvalidTopicsAndMalformedStream(t *testing.T) {
	asHQ(t)
	for _, data := range [][]byte{
		[]byte("{\"v\":3,\"op\":\"add\",\"id\":\"pitfalls/x\",\"topic\":\"pitfalls\",\"future_field\":true}\n"),
		[]byte("{\"v\":3,\"op\":\"add\",\"id\":\"pitfalls/x\",\"topic\":\"pitfalls\"} {}\n"),
		migrationData(t, knowledgeOp{Op: knowledgeOpAdd, ID: "../x", Topic: "..", Title: "unsafe"}),
		migrationData(t, knowledgeOp{Op: knowledgeOpAdd, ID: "workflows/x", Topic: "pitfalls", Title: "wrong prefix"}),
		[]byte{0xff},
	} {
		if _, _, err := MigrationEntries(data, false); err == nil {
			t.Fatalf("invalid source accepted: %s", data)
		}
	}
	if _, err := os.Stat(knowledgeLedgerPath()); !os.IsNotExist(err) {
		t.Fatal("invalid preview wrote destination")
	}
}

func TestMigrationRetainsBothLanguageHalvesAndCustomTopic(t *testing.T) {
	asHQ(t)
	data := migrationData(t,
		knowledgeOp{Op: knowledgeOpTopic, ID: "personal-lessons", Title: "Personal lessons"},
		knowledgeOp{Op: knowledgeOpAdd, ID: "personal-lessons/one", Topic: "personal-lessons", Title: "A lesson", Body: "Source body", Lang: "en", Alt: &altHalf{Lang: "zh", Title: "一条经验", Body: "中文正文"}},
	)
	preview, _, err := MigrationEntries(data, false)
	if err != nil || len(preview) != 1 || preview[0].Alt == nil || preview[0].Alt.Title != "一条经验" {
		t.Fatalf("bilingual preview: %+v %v", preview, err)
	}
	if _, err := ApplyMigration(data, []string{"personal-lessons/one"}, "archive"); err != nil {
		t.Fatal(err)
	}
	live, _ := liveKnowledge()
	if len(live) != 1 || live[0].Lang != "en" || live[0].Alt == nil || live[0].Alt.Body != "中文正文" {
		t.Fatal("migration lost language halves")
	}
}
