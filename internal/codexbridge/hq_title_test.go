package codexbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	"github.com/gorilla/websocket"
)

func titleObserverFixture(t *testing.T, directory string, epoch *connectionEpoch, id uint64, publish func(string)) (*frontendBinding, *observer) {
	t.Helper()
	b := newConnectionBinding(context.Background(), resume.CodexFrontend{
		HomeID: transcript.CodexHomeIDForPath(directory), Sequence: id,
	}, epoch, nil)
	b.end = func(resume.CodexFrontend) {}
	t.Cleanup(b.close)
	b.bind = func(f resume.CodexFrontend, thread, _ string) (resume.CodexFrontend, error) {
		f.SessionID = thread
		publish(thread)
		return f, nil
	}
	o := b.observer()
	o.request([]byte(`{"id":0,"method":"initialize"}`))
	home, _ := json.Marshal(directory)
	o.response([]byte(`{"id":0,"result":{"codexHome":` + string(home) + `}}`))
	if !o.homeVerified() {
		t.Fatal("fixture home verification failed")
	}
	return b, o
}

// Exercise the same forwarding and observer wiring as the accepted frontend
// transport, with the backend response deliberately delayed by the caller.
func checkHQTitleFrames(t *testing.T, o *observer, frames ...[]byte) {
	t.Helper()
	client, down := trustUnixPair(t)
	up, backend := trustUnixPair(t)
	done := make(chan error, 1)
	go func() { done <- forward(up, down, o.response) }()
	defer func() {
		_ = up.Close()
		<-done
	}()
	for _, want := range frames {
		if err := backend.WriteMessage(websocket.TextMessage, want); err != nil {
			t.Fatal(err)
		}
		kind, got, err := client.ReadMessage()
		if err != nil || kind != websocket.TextMessage || !bytes.Equal(got, want) {
			t.Fatalf("Codex frame changed after forwarding: got %s, want %s, error %v", got, want, err)
		}
	}
}

func TestHCOTitleFramesKeepNativeRenameAfterStaleRoot(t *testing.T) {
	var epoch connectionEpoch
	id := epoch.reserve()
	epoch.activate(id)
	var commits []string
	_, o := titleObserverFixture(t, t.TempDir(), &epoch, id, func(thread string) { commits = append(commits, thread) })
	for i, method := range []string{"thread/start", "thread/resume", "thread/start"} {
		thread := fmt.Sprintf("root-%d", i+1)
		o.request([]byte(fmt.Sprintf(`{"id":%d,"method":%q}`, i+1, method)))
		// The server prepared this result while the root was unnamed. Its
		// notification reflects the independently confirmed later rename.
		stale := []byte(fmt.Sprintf(`{"id":%d,"result":{"thread":{"id":%q,"name":null,"source":"cli","turns":[]}}}`, i+1, thread))
		rename := []byte(fmt.Sprintf(`{"method":"thread/name/updated","params":{"threadId":%q,"threadName":"My coordination title"}}`, thread))
		checkHQTitleFrames(t, o, stale, rename)
	}
	if !slices.Equal(commits, []string{"root-1", "root-2", "root-3"}) {
		t.Fatalf("unexpected root commits: %v", commits)
	}
}

func TestHCORetiredFrontendCannotReclaimDelayedRoot(t *testing.T) {
	var epoch connectionEpoch
	first, second := epoch.reserve(), epoch.reserve()
	epoch.activate(first)
	directory := t.TempDir()
	var commits []string
	publish := func(thread string) { commits = append(commits, thread) }
	b, old := titleObserverFixture(t, directory, &epoch, first, publish)
	old.request([]byte(`{"id":1,"method":"thread/start"}`))
	stale := []byte(`{"id":1,"result":{"thread":{"id":"old-root","name":null,"source":"cli"}}}`)
	ended := make(chan error, 2)
	ended <- nil
	release := make(chan struct{})
	go func() {
		<-release
		if !epoch.activate(second) {
			t.Error("reconnect could not activate")
		}
		old.response(stale)
		ended <- nil
	}()
	down := bindingLifetimeCloser(func() error { return nil })
	up := bindingLifetimeCloser(func() error { close(release); return nil })
	runFrontendLifetime(context.Background(), nil, down, up, ended, b)

	_, current := titleObserverFixture(t, directory, &epoch, second, publish)
	current.request([]byte(`{"id":2,"method":"thread/start"}`))
	checkHQTitleFrames(t, current, rootResponse(2, "current-root"))
	if !slices.Equal(commits, []string{"current-root"}) {
		t.Fatalf("retired root reached publication: %v", commits)
	}
}
