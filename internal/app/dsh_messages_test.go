package app

import (
	"encoding/json"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/dshfeed"
	"os"
	"path/filepath"
	"testing"
)

func TestDSHMessageCLISeparatesReadFromReview(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	dir := filepath.Join(dsh.Home(), "sessions", "work", "synthetic")
	_ = os.MkdirAll(dir, 0700)
	_ = os.WriteFile(filepath.Join(dir, "session.v4.jsonl"), []byte(`{"type":"session","version":4,"id":"synthetic"}`+"\n"+`{"type":"user/message","seq":1,"time":2000,"data":{"role":"user","source":{"kind":"user"},"content":[{"type":"text","text":"synthetic public text"}]}}`+"\n"), 0600)
	output := captureStdout(t, func() {
		if cmdTranscript([]string{"synthetic", "--messages", "--json"}) != 0 {
			t.Fatal("message read failed")
		}
	})
	var page dsh.MessagePage
	if json.Unmarshal([]byte(output), &page) != nil || len(page.Messages) != 1 || page.ReviewedThrough != -1 {
		t.Fatal("read changed review or missed message", output)
	}
	if _, err := dshfeed.Reviewed(dsh.HomeID(), "synthetic"); err != nil {
		t.Fatal(err)
	}
	captureStdout(t, func() {
		if cmdTranscript([]string{"synthetic", "--messages", "--review-through", "1", "--json"}) != 0 {
			t.Fatal("explicit review failed")
		}
	})
	if n, _ := dshfeed.Reviewed(dsh.HomeID(), "synthetic"); n != 1 {
		t.Fatal("review not declared")
	}
	for _, args := range [][]string{{"synthetic", "--messages", "--after-seq", "4", "--review-through", "5"}, {"synthetic", "--messages", "--limit", "0"}, {"synthetic", "--messages", "--home-id", "foreign"}, {"synthetic", "--messages", "--etag", "bad"}} {
		captureStdout(t, func() {
			if cmdTranscript(args) == 0 {
				t.Fatal("invalid message arguments accepted", args)
			}
		})
	}
}
