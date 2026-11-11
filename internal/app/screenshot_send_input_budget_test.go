package app

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/screenshot"
)

func TestASC009StalledNoteHasNoClaimOrSendAfterDeadline(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	target := screenshot.Target{Version: 1, Pane: "%9", Loc: "fixture:0.0", Agent: "codex", SessionID: "s", HomeID: "h", Instance: "i", ClientPID: 100, PanePID: 99, ServerPID: 90, ServerStart: "birth"}
	token, err := screenshot.MintTarget(target, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	oldTarget, oldSave, oldStdin := screenshotCurrentTarget, screenshotSaveAttachment, os.Stdin
	t.Cleanup(func() { screenshotCurrentTarget, screenshotSaveAttachment, os.Stdin = oldTarget, oldSave, oldStdin })
	screenshotCurrentTarget = func(string) (screenshot.Target, error) { return target, nil }
	saved := 0
	screenshotSaveAttachment = func(string, []byte) (string, error) { saved++; return "", nil }
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "in.png")
	if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	os.Stdin = reader
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	type result struct {
		receipt screenshot.Receipt
		err     error
	}
	done := make(chan result, 1)
	go func() { r, e := screenshotSend(ctx, path, token, "budgeted-note-0001", "-"); done <- result{r, e} }()
	var got result
	select {
	case got = <-done:
	case <-time.After(500 * time.Millisecond):
		writer.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("note control did not settle")
		}
		t.Fatal("stalled stdin exceeded the send deadline")
	}
	if got.err == nil || got.receipt.ID != "" || saved != 0 {
		t.Fatal("expired note created delivery state", got, saved)
	}
	if _, err := screenshot.ReadReceipt("budgeted-note-0001"); err == nil {
		t.Fatal("expired note left a claim")
	}
	if _, err := reader.Stat(); err != nil {
		t.Fatal("original stdin closed", err)
	}
}
