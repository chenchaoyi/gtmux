package hq

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The index is what a remote client polls: identities and state, never bodies. A base of
// a few hundred entries with bodies is megabytes over a tunnel.
func TestKnowledgeIndexCarriesStateNotBodies(t *testing.T) {
	asHQ(t)
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "office TLS resets"}); rc != 0 {
		t.Fatal("add failed")
	}
	if rc := CmdKnowledge([]string{"add", "--topic", "workflows", "--title", "tag then verify"}); rc != 0 {
		t.Fatal("add failed")
	}
	idx := KnowledgeIndex(time.Now().Unix())
	if len(idx.Entries) != 2 {
		t.Fatalf("2 entries added, index has %d", len(idx.Entries))
	}
	// Newest first: the review question on a phone is "what did it just learn".
	if idx.Entries[0].Title != "tag then verify" {
		t.Errorf("index is not newest-first: %q leads", idx.Entries[0].Title)
	}
	b, _ := json.Marshal(idx)
	if strings.Contains(string(b), `"body"`) {
		t.Error("the index must not carry bodies")
	}
	byName := map[string]int{}
	for _, tp := range idx.Topics {
		byName[tp.Name] = tp.Count
	}
	if byName["pitfalls"] != 1 || byName["workflows"] != 1 {
		t.Errorf("topic counts wrong: %v", byName)
	}
	if len(idx.Topics) < len(builtinTopics) {
		t.Errorf("the whole vocabulary must be reported, got %d topics", len(idx.Topics))
	}
}

// A Mac with no HQ home is an ordinary machine, not an error a client must handle.
func TestKnowledgeIndexOnAnEmptyBase(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())
	idx := KnowledgeIndex(time.Now().Unix())
	if idx.Entries == nil || len(idx.Entries) != 0 {
		t.Errorf("an empty base must be an empty slice, not nil or entries: %#v", idx.Entries)
	}
	if idx.Promotions.Pending != 0 || idx.Candidates.Pending != 0 {
		t.Error("an empty base owes nothing")
	}
	if _, ok := KnowledgeEntry("pitfalls/nope"); ok {
		t.Error("no entry can be found in an empty base")
	}
}

// The detail read is the only place a body travels.
func TestKnowledgeEntryCarriesTheBody(t *testing.T) {
	asHQ(t)
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "office TLS resets"}); rc != 0 {
		t.Fatal("add failed")
	}
	id := "pitfalls/" + slug("office TLS resets")
	e, ok := KnowledgeEntry(id)
	if !ok {
		t.Fatalf("entry %q not found", id)
	}
	if e.ID != id || e.Topic != "pitfalls" {
		t.Errorf("wrong entry: %+v", e)
	}
}

// The two remote verbs are the SAME verbs the CLI runs — one implementation of what
// landing and retiring mean, two doors onto it.
func TestKnowledgeRemoteVerbsMatchTheCLI(t *testing.T) {
	asHQ(t)
	if rc := CmdKnowledge([]string{"add", "--topic", "pitfalls", "--title", "office TLS resets"}); rc != 0 {
		t.Fatal("add failed")
	}
	id := "pitfalls/" + slug("office TLS resets")

	// Landing something that was never promoted is refused, and changes nothing.
	before, _ := readKnowledgeOps()
	if err := KnowledgeLand(id, "PR #1"); err == nil {
		t.Error("landing an unpromoted entry must fail")
	}
	if after, _ := readKnowledgeOps(); len(after) != len(before) {
		t.Error("a refused land appended to the ledger")
	}

	if rc := CmdKnowledge([]string{"promote", id, "--why", "it governs every release"}); rc != 0 {
		t.Fatal("promote failed")
	}
	if idx := KnowledgeIndex(time.Now().Unix()); idx.Promotions.Pending != 1 {
		t.Fatalf("promotion queue says %d pending", idx.Promotions.Pending)
	}
	if err := KnowledgeLand(id, "AGENTS.md"); err != nil {
		t.Fatalf("land: %v", err)
	}
	idx := KnowledgeIndex(time.Now().Unix())
	if idx.Promotions.Pending != 0 {
		t.Error("landing must clear the queue")
	}
	if idx.Entries[0].LandedRef != "AGENTS.md" {
		t.Errorf("the landing ref must survive on the entry: %+v", idx.Entries[0])
	}

	// Both mutations journal, exactly as the CLI's do.
	audits := knowledgeAudits(t)
	if len(audits) != 3 || !strings.Contains(audits[2], "land "+id) {
		t.Fatalf("land must journal like every other mutation: %v", audits)
	}

	if err := KnowledgeRetire(id, ""); err == nil {
		t.Error("retire without a reason must fail")
	}
	if err := KnowledgeRetire(id, "the office network was fixed"); err != nil {
		t.Fatalf("retire: %v", err)
	}
	if _, ok := KnowledgeEntry(id); ok {
		t.Error("a retired entry must not be live")
	}
	if err := KnowledgeRetire(id, "again"); err == nil {
		t.Error("retiring what is already gone must fail")
	}
}
