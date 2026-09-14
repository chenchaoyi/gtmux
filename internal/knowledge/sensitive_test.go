package knowledge

import (
	"strings"
	"testing"
)

// kb-sensitive-entries: the commander's own detail may go in, after they confirm, and
// then it stays on this machine. These pin the mechanism the charter's rule rests on.

func TestASensitiveWriteNeedsTheCommandersWords(t *testing.T) {
	asHQ(t)
	err := knowledgeAdd([]string{"--topic", "accounts", "--title", "my registry login", "--sensitive"})
	if err == nil || !strings.Contains(err.Error(), "--confirmed") {
		t.Fatalf("a sensitive add without --confirmed must be refused, got %v", err)
	}
	if err := knowledgeAdd([]string{"--topic", "accounts", "--title", "my registry login", "--sensitive", "--confirmed", "可以，记下"}); err != nil {
		t.Fatalf("add with confirmation: %v", err)
	}
	live, err := liveKnowledge()
	if err != nil || len(live) != 1 {
		t.Fatalf("live = %v %v", live, err)
	}
	if !live[0].Sensitive || live[0].Confirmed != "可以，记下" {
		t.Fatalf("the mark and the words must be on the entry: %+v", live[0])
	}
	// The API row carries the mark; a reader's surface shows the lock from it.
	if !rowOf(live[0]).Sensitive {
		t.Error("the API row lost the mark")
	}
}

func TestASensitiveEntryDoesNotTravel(t *testing.T) {
	asHQ(t)
	if err := knowledgeAdd([]string{"--topic", "accounts", "--title", "my registry login", "--sensitive", "--confirmed", "yes"}); err != nil {
		t.Fatal(err)
	}
	live, _ := liveKnowledge()
	id := live[0].ID
	for _, aud := range []string{"machine", "everyone", "repo:/tmp/x"} {
		err := knowledgePromote([]string{id, "--why", "handy", "--for", aud})
		if err == nil || !strings.Contains(err.Error(), "sensitive") {
			t.Errorf("promote --for %s must be refused for a sensitive entry, got %v", aud, err)
		}
	}
	if err := knowledgePromote([]string{id, "--why", "for my own charter", "--for", "hq"}); err != nil {
		t.Errorf("--for hq is the one audience a sensitive entry may have: %v", err)
	}
	// Belt and braces: even an entry that somehow carries the machine audience is left
	// out of what every agent reads.
	ops := []knowledgeOp{{Op: knowledgeOpAdd, ID: "accounts/x", Topic: "accounts", Title: "secret-ish", Body: "b", At: 1,
		Audience: AudienceMachine, PromotedAt: 5, Sensitive: true, Lang: "en"}}
	if idx := machineIndex(foldKnowledge(ops)); strings.Contains(idx, "secret-ish") {
		t.Errorf("machine.md carried a sensitive entry:\n%s", idx)
	}
}

func TestSupersedeKeepsTheMarkAndTheVerbFlipsIt(t *testing.T) {
	asHQ(t)
	if err := knowledgeAdd([]string{"--topic", "accounts", "--title", "my registry login", "--sensitive", "--confirmed", "yes"}); err != nil {
		t.Fatal(err)
	}
	live, _ := liveKnowledge()
	if err := knowledgeSupersede([]string{live[0].ID, "--title", "my registry login, renewed"}); err != nil {
		t.Fatal(err)
	}
	live, _ = liveKnowledge()
	if len(live) != 1 || !live[0].Sensitive || live[0].Confirmed != "yes" {
		t.Fatalf("a rewritten sensitive lesson is still sensitive: %+v", live)
	}
	if err := knowledgeSensitive([]string{live[0].ID, "--off"}); err == nil {
		t.Error("unmarking without the commander's words must be refused")
	}
	if err := knowledgeSensitive([]string{live[0].ID, "--off", "--confirmed", "it may travel now"}); err != nil {
		t.Fatal(err)
	}
	live, _ = liveKnowledge()
	if live[0].Sensitive {
		t.Error("--off did not unmark")
	}
}

func TestLintNamesACredentialWrittenWithoutAsking(t *testing.T) {
	ops := []knowledgeOp{
		{Op: knowledgeOpAdd, ID: "accounts/a", Topic: "accounts", Title: "registry", Body: "token=abcdef123456 for the mirror", At: 1, Lang: "en"},
		{Op: knowledgeOpAdd, ID: "accounts/b", Topic: "accounts", Title: "registry", Body: "password=hunter2hunter2", At: 1, Lang: "en", Sensitive: true, Confirmed: "yes"},
		{Op: knowledgeOpAdd, ID: "pitfalls/c", Topic: "pitfalls", Title: "the password prompt hangs", Body: "when the password field is empty the CLI waits forever — no secret here", At: 1, Lang: "en"},
	}
	rep := lint(ops, 10)
	got := findings(rep, "unmarked-sensitive")
	if len(got) != 1 || got[0].ID != "accounts/a" {
		t.Errorf("unmarked-sensitive = %+v, want only accounts/a (b is marked, c only mentions the word)", got)
	}
	for _, s := range []string{"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", "-----BEGIN RSA PRIVATE KEY-----", "sk-abcdefghijklmnopqrstuvwxyz", "AKIAABCDEFGHIJKLMNOP"} {
		if !looksLikeCredential(s) {
			t.Errorf("%q should read as a credential", s)
		}
	}
	for _, s := range []string{"reset your password in Settings", "the token bucket refills hourly", "api key rotation is monthly"} {
		if looksLikeCredential(s) {
			t.Errorf("%q should not read as a credential", s)
		}
	}
}
