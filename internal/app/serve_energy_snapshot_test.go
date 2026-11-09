package app

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

func TestPWR003SSEPublishesItsActualNativeObservationForREST(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	saved := tmux.Bin
	tmux.Bin = ""
	t.Cleanup(func() { tmux.Bin = saved })
	rec := native.Record{Agent: "claude", SessionID: "synthetic-energy", Cwd: t.TempDir(), State: "idle", UpdatedAt: time.Now().Unix(), PID: os.Getpid()}
	if err := native.Save(rec); err != nil {
		t.Fatal(err)
	}
	sse := serveAgentStatuses()
	if len(sse) != 1 || sse[0].SessionID != rec.SessionID || sse[0].Status != "idle" {
		t.Fatalf("first SSE: %+v", sse)
	}
	rec.State = "working"
	if err := native.Save(rec); err != nil {
		t.Fatal(err)
	}
	assertREST := func(want string) {
		t.Helper()
		b, e := radar.CachedAgentsJSONBytes()
		if e != nil {
			t.Fatal(e)
		}
		var rows []struct {
			Status, Source string
			SessionID      string `json:"session_id"`
		}
		if e = json.Unmarshal(b, &rows); e != nil {
			t.Fatal(e)
		}
		if len(rows) != 1 || rows[0].Status != want || rows[0].Source != "native" || rows[0].SessionID != rec.SessionID {
			t.Fatalf("REST repeated collection instead of sharing %s: %s", want, b)
		}
	}
	assertREST("idle")
	sse = serveAgentStatuses()
	if len(sse) != 1 || sse[0].Status != "working" {
		t.Fatalf("fresh SSE suppressed: %+v", sse)
	}
	assertREST("working")
}
