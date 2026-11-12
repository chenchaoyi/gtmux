package knowledge

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestMemoryCarrierUpdatesHistoricalTextAndPreservesManualEdits(t *testing.T) {
	e := reviewFixture(t)
	os.WriteFile(LocalPath(), []byte("# operator conventions\n"), 0600)
	if _, err := carryIntoLocal(e); err != nil {
		t.Fatal(err)
	}
	p := proposalFor(e)
	if _, err := ApplyReview(p, false); err != nil {
		t.Fatal(err)
	}
	live, _ := liveKnowledge()
	next := live[0]
	if _, err := carryIntoLocal(next); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(LocalPath())
	if strings.Contains(string(b), e.Body) || !strings.Contains(string(b), next.Body) || !strings.HasPrefix(string(b), "# operator conventions") {
		t.Fatalf("wrong section: %s", b)
	}
	if _, err := carryIntoLocal(next); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(LocalPath())
	if !bytes.Equal(b, again) {
		t.Fatal("duplicate landing")
	}
	edited := strings.Replace(string(b), next.Body, "human clarification", 1)
	os.WriteFile(LocalPath(), []byte(edited), 0600)
	if _, err := carryIntoLocal(next); err == nil {
		t.Fatal("overwrote operator edit")
	}
	after, _ := os.ReadFile(LocalPath())
	if string(after) != edited {
		t.Fatal("conflict changed carrier")
	}
}
