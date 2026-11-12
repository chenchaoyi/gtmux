package radar

import (
	"encoding/json"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/transcript"
	"os"
	"testing"
	"time"
)

func TestSID004HistoricalProjectionIsUniqueAndDoesNotBorrowAuthority(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	now := time.Now().Unix()
	home := transcript.CodexHomeID()
	for _, sid := range []string{"history", "same-daemon-unrelated"} {
		writeNativeCodexRollout(t, os.Getenv("CODEX_HOME"), sid)
		if err := native.Save(native.Record{Agent: "codex", SessionID: sid, HomeID: home, State: "working", UpdatedAt: now, PID: os.Getpid()}); err != nil {
			t.Fatal(err)
		}
	}
	original, _ := native.Load("history", home)
	old := observedConversations
	t.Cleanup(func() { observedConversations = old })
	observedConversations = func(*transcript.CodexPathScope, map[string]string, int64) map[string]resume.ConversationObservation {
		return map[string]resume.ConversationObservation{"%fixture": {SessionID: "history", HomeID: home, State: "last-known"}}
	}
	panes := []Pane{{PaneID: "%fixture", Loc: "synthetic:0.0", Agent: "Codex", Status: "running", Task: "existing terminal", Since: now - 300}}
	more := nativePanesScoped(panes, nil, now, transcript.NewCodexPathScope())
	if len(more) != 1 || more[0].sessionID != "same-daemon-unrelated" {
		t.Fatalf("wrong history folded: %+v", more)
	}
	p := panes[0]
	if p.sessionID != "" || p.homeID != "" || p.Status != "running" || p.Task != "existing terminal" || p.Since != now-300 || p.observedConversation == nil {
		t.Fatalf("projection gained lifecycle/identity authority: %+v", p)
	}
	b, err := agentsJSONBytes(panes)
	var rows []map[string]any
	if err != nil || json.Unmarshal(b, &rows) != nil || len(rows) != 1 || rows[0]["session_id"] != nil || rows[0]["observed_conversation"] == nil {
		t.Fatal(string(b), err)
	}
	after, _ := native.Load("history", home)
	if after != original {
		t.Fatal("projection pruned/changed native history")
	}
	observedConversations = func(*transcript.CodexPathScope, map[string]string, int64) map[string]resume.ConversationObservation {
		return nil
	}
	panes[0].observedConversation = nil
	if got := nativePanesScoped(panes, nil, now, transcript.NewCodexPathScope()); len(got) != 2 {
		t.Fatalf("missing proof hid genuine history: %+v", got)
	}
}
