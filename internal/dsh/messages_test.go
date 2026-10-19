package dsh

import (
	"encoding/json"
	"testing"
)

func TestPublicMessagePagesExcludePrivateAndReplacementEvents(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	secret := `{"type":"assistant/message","seq":2,"time":2100,"surfaceOp":"replace","data":{"message":{"role":"assistant","content":[{"type":"text","text":"private replacement"}]}}}` + "\n"
	response := `{"type":"assistant/message","seq":3,"time":2200,"data":{"message":{"role":"assistant","content":[{"type":"reasoning","text":"private reasoning"},{"type":"text","text":"public answer"}]}}}` + "\n"
	artifact(t, "ws", "messages", "", head("messages")+promptEvent+secret+response)
	page, err := ReadMessages("messages", -1, 1)
	if err != nil || len(page.Messages) != 1 || !page.HasMore || page.NextSeq != 1 {
		t.Fatalf("first page %+v %v", page, err)
	}
	page, err = ReadMessages("messages", page.NextSeq, 100)
	if err != nil || len(page.Messages) != 1 || page.Messages[0].Text != "public answer" || page.HasMore {
		t.Fatalf("second page %+v %v", page, err)
	}
	var event Event
	_ = json.Unmarshal([]byte(`{"type":"user/message","seq":4,"data":{"role":"user","source":{"kind":"system"},"content":[{"type":"text","text":"private injection"}]}}`), &event)
	if _, public, _ := PublicMessage(event); public {
		t.Fatal("system injection exposed")
	}
}
