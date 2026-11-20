package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/usercfg"
)

func TestReplyRequiresOriginalProducerAndRefusalOrder(t *testing.T) {
	for _, scenario := range []string{
		"legacy-first-verification", "legacy-later-generation", "legacy-new-confirmation",
		"final-no-cursor-earlier", "final-no-cursor-later", "final-no-cursor-message",
		"final-delayed-source", "final-same-second", "final-delayed-fresh",
	} {
		t.Run(scenario, func(t *testing.T) {
			if strings.HasPrefix(scenario, "legacy-") {
				exerciseLegacyProducer(t, scenario)
				return
			}
			exerciseReplyCoverage(t, scenario)
		})
	}
}

func exerciseLegacyProducer(t *testing.T, scenario string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(state.MkdirAll(state.ConfigDir(), 0700))
	must(os.WriteFile(usercfg.Path(), []byte(`{"eventsCapMB":20}`), 0600))
	must(state.MkdirAll(state.Dir(), 0700))
	// The older writer did not leave a hole if it could not allocate a pause.
	// Its retained rows and watermark are contiguous, and config predates them.
	at := time.Now().Unix() + 2
	c := hqworkspace.Conversation{Agent: "codex", HomeID: "fixture-home", SessionID: "fixture-target"}
	old := events.Record{Ts: at, Seq: 1, Event: "UserPromptSubmit", Origin: events.OriginInstruction, Agent: "Codex", HomeID: c.HomeID, AgentSession: c.SessionID}
	stop := events.Record{Ts: at + 2, Seq: 2, Event: "Stop", Agent: "Codex", AgentSession: "other"}
	f, err := os.Create(events.Path())
	must(err)
	enc := json.NewEncoder(f)
	must(enc.Encode(old))
	must(enc.Encode(stop))
	must(f.Close())
	must(os.WriteFile(filepath.Join(state.Dir(), "events.seq"), []byte("2"), 0600))
	if scenario != "legacy-first-verification" {
		stop.Seq = 0
		persistReplyEvents(t, stop)
	}
	_, err = workspaceVerifiedUserEvent("events:1")
	if err == nil {
		t.Error("accepted a direct source lacking contemporaneous coverage")
	}
	cursor := int64(1)
	d := hqworkspace.Empty()
	d.Matters["work"] = hqworkspace.Matter{ID: "work", Responsibility: &hqworkspace.Responsibility{AuthorityRef: "message:old"}}
	d.Messages["old"] = hqworkspace.Message{ID: "old", MatterID: "work", At: at + 1, EventCursor: &cursor}
	a := hqworkspace.IntentAttempt{MatterID: "work", Intent: hqworkspace.ActionIntent{Target: c}}
	if workspaceMessageHistoryHold(d, a) == "" {
		t.Error("accepted unmarked workspace authority")
	}
	if scenario == "legacy-first-verification" {
		if _, err := os.Stat(filepath.Join(state.StatusDir(), "event-coverage")); !os.IsNotExist(err) {
			t.Fatal("verification minted a generation", err)
		}
	}
	if scenario == "legacy-new-confirmation" {
		old.Seq = 0
		old.Ts = at + 4
		persistReplyEvents(t, old)
		if fresh, err := workspaceVerifiedUserEvent("events:4"); err != nil || fresh == nil || fresh.Seq != 4 {
			t.Fatal("new producer authority unavailable", fresh, err)
		}
	}
}
