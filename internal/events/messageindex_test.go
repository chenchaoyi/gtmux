package events

import (
	"os"
	"strings"
	"testing"
)

func TestPublicMessageIndexAppendPartialAndRotation(t *testing.T) {
	withJournal(t, []Record{{MessageID: "first", Event: "ConversationMessage"}})
	x := &MessageIndex{}
	if !x.Contains("first") {
		t.Fatal("missing key")
	}
	offset := x.offset
	x.Contains("first")
	if x.offset != offset {
		t.Fatal("unchanged cursor moved")
	}
	f, _ := os.OpenFile(Path(), os.O_APPEND|os.O_WRONLY, 0600)
	_, _ = f.WriteString(`{"message_id":"second"}`)
	_ = f.Close()
	if x.Contains("second") {
		t.Fatal("partial line indexed")
	}
	f, _ = os.OpenFile(Path(), os.O_APPEND|os.O_WRONLY, 0600)
	_, _ = f.WriteString("\n")
	_ = f.Close()
	if !x.Contains("second") {
		t.Fatal("complete append missing")
	}
	if err := os.Rename(Path(), rotatedPath()); err != nil {
		t.Fatal(err)
	}
	appendRecords(t, Path(), Record{MessageID: "third"})
	if !x.Contains("first") || !x.Contains("third") {
		t.Fatal("rotation lost keys")
	}
	if err := os.WriteFile(Path(), []byte(strings.Repeat("x", 3<<20)+"\n"+`{"message_id":"last"}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if !x.Contains("last") {
		t.Fatal("oversized unrelated line stranded cursor")
	}
}
