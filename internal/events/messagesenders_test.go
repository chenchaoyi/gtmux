package events

import (
	"os"
	"strings"
	"testing"
)

func publicPrompt(seq, ts, messageSeq int64, home, session, text string) Record {
	return Record{Seq: seq, Ts: ts, Event: "ConversationMessage", Pane: "%19", AgentKey: "dsh", HomeID: home, AgentSession: session, MessageID: "synthetic", MessageSeq: messageSeq, MessageRole: "user", Summary: text}
}

func TestMessageSendersSurvivePaneClosureAndReadOnlyAppends(t *testing.T) {
	withJournal(t, []Record{send(1, 1000, "%19", "hq", "Inspect the synthetic project")})
	x := &SendIndex{}
	before, revision := x.MessageSenders("dsh", "home", "session")
	if len(before) != 0 {
		t.Fatal("a delivery alone does not prove a session")
	}
	appendRecords(t, Path(), publicPrompt(2, 1002, 0, "home", "session", "Inspect the synthetic project"))
	got, next := x.MessageSenders("dsh", "home", "session")
	if got[0].Actor != "hq" || next == revision || !got[0].Matches("Inspect the synthetic project", 1001) {
		t.Fatalf("lost exact native message or sender revision: %+v %s %s", got, revision, next)
	}
	if got[0].Matches("Unrelated instruction", 1001) || got[0].Matches("Inspect the synthetic project", 5000) {
		t.Fatal("a rewritten or later message borrowed historical attribution")
	}
	parsed := x.parsed
	got[0] = MessageSender{Actor: "changed by reader"}
	got, same := x.MessageSenders("dsh", "home", "session")
	if x.parsed != parsed || same != next || got[0].Actor != "hq" {
		t.Fatal("unchanged polling reparsed records or exposed the cached map")
	}
}

func TestMessageSendersKeepRepeatedWordsAndScopesSeparate(t *testing.T) {
	withJournal(t, []Record{
		publicPrompt(1, 1000, 1, "home", "first", "Continue"), send(2, 1001, "%19", "hq", "Continue"),
		publicPrompt(3, 2000, 2, "home", "first", "Continue"),
		publicPrompt(4, 3000, 3, "home", "first", "Continue"), send(5, 3001, "%19", "agent:%31", "Continue"),
		publicPrompt(6, 4000, 1, "other-home", "first", "Continue"),
		publicPrompt(7, 5000, 1, "home", "other-session", "Continue"),
	})
	x := &SendIndex{}
	by, _ := x.MessageSenders("dsh", "home", "first")
	if len(by) != 2 || by[1].Actor != "hq" || by[3].Actor != "agent:%31" {
		t.Fatalf("repeated human/agent prompts borrowed the latest sender: %+v", by)
	}
	for _, scope := range []messageScope{{"dsh", "other-home", "first"}, {"dsh", "home", "other-session"}, {"codex", "home", "first"}, {"dsh", "", "first"}} {
		if got, _ := x.MessageSenders(scope.agent, scope.home, scope.session); len(got) != 0 {
			t.Fatalf("scope leaked: %+v %+v", scope, got)
		}
	}
}

func TestMessageSendersRefuseFailuresAmbiguityAndSequenceReuse(t *testing.T) {
	recs := []Record{
		publicPrompt(1, 1000, 1, "home", "session", "Failed instruction"),
		{Seq: 2, Ts: 1001, Event: AuditEventSend, Pane: "%19", Actor: "hq", Summary: "failed: Failed instruction"},
		publicPrompt(3, 2000, 2, "home", "session", "Refused instruction"),
		{Seq: 4, Ts: 2001, Event: AuditEventSend, Pane: "%19", Actor: "hq", Summary: "refused-draft: Refused instruction"},
		publicPrompt(5, 3000, 3, "home", "session", "Tied instruction"),
		send(6, 3001, "%19", "hq", "Tied instruction"), send(7, 3001, "%19", "agent:%31", "Tied instruction"),
		publicPrompt(8, 4000, 4, "home", "session", "Rewritten instruction"), send(9, 4001, "%19", "hq", "Rewritten instruction"),
		publicPrompt(10, 5000, 4, "home", "session", "Rewritten instruction"),
	}
	withJournal(t, recs)
	if got, _ := (&SendIndex{}).MessageSenders("dsh", "home", "session"); len(got) != 0 {
		t.Fatalf("unproven or reused message received a sender: %+v", got)
	}
}

func TestMessageSendersFollowRotationRewriteAndMissingJournal(t *testing.T) {
	withJournal(t, []Record{send(1, 1000, "%19", "hq", "Inspect this")})
	x := &SendIndex{}
	x.MessageSenders("dsh", "home", "session")
	if err := os.Rename(Path(), rotatedPath()); err != nil {
		t.Fatal(err)
	}
	appendRecords(t, Path(), publicPrompt(2, 1001, 1, "home", "session", "Inspect this"))
	got, _ := x.MessageSenders("dsh", "home", "session")
	if got[1].Actor != "hq" {
		t.Fatal("rotation lost a delivery from the previous generation")
	}
	if err := os.Remove(rotatedPath()); err != nil {
		t.Fatal(err)
	}
	bytes, _ := os.ReadFile(Path())
	if err := os.WriteFile(Path(), []byte(strings.ReplaceAll(string(bytes), "Inspect this", "Other words!")), 0600); err != nil {
		t.Fatal(err)
	}
	if got, _ := x.MessageSenders("dsh", "home", "session"); len(got) != 0 {
		t.Fatal("same-size rewrite retained previous sender evidence")
	}
	if err := os.Remove(Path()); err != nil {
		t.Fatal(err)
	}
	if got, _ := x.MessageSenders("dsh", "home", "session"); len(got) != 0 {
		t.Fatal("a missing journal retained stale evidence")
	}
}

func TestMessageSenderEvidenceIsBounded(t *testing.T) {
	x := &SendIndex{}
	for i := int64(1); i <= sendWitnessLimit+20; i++ {
		x.keepWitness(send(i, i, "%19", "hq", "Synthetic instruction"))
	}
	if len(x.witnesses) != sendWitnessLimit {
		t.Fatalf("unbounded witness history: %d", len(x.witnesses))
	}
}
