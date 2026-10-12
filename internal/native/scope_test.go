package native

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestScopedObservationsAndRetirementDoNotCrossHomes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	a, b := strings.Repeat("a", 32), strings.Repeat("b", 32)
	original := Record{Agent: "codex", SessionID: "same-thread", HomeID: a, State: "idle", Event: "SessionStart", UpdatedAt: now, Cwd: "/project"}
	other := original
	other.HomeID = b
	other.State = "working"
	other.PID = os.Getpid()
	for _, r := range []Record{original, other} {
		if err := Save(r); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := Load(original.SessionID); ok {
		t.Fatal("ambiguous scope resolved by recency")
	}
	snapshot, ok := Load(original.SessionID, a)
	if !ok {
		t.Fatal("missing scoped observation")
	}
	proof := Binding{HomeID: a, ID: "binding", SessionID: original.SessionID, Pane: "%9", Loc: "worker:0.0", Cwd: "/project", PID: 100, PanePID: 99, PreparedAt: now, BoundAt: now}
	if err := ConfirmBinding(proof, snapshot); err != nil {
		t.Fatal(err)
	}
	if _, ok := Load(original.SessionID, a); ok {
		t.Fatal("exact observation not retired")
	}
	if got, ok := Load(original.SessionID, b); !ok || got.State != "working" {
		t.Fatal("other home was retired", got)
	}
	if _, ok := BindingFor(original.SessionID, b); ok {
		t.Fatal("binding crossed homes")
	}
	if got := Live(now); len(got) != 1 || got[0].HomeID != b {
		t.Fatal("reconciliation crossed scopes", got)
	}
	if err := Save(Record{SessionID: "unsafe", HomeID: "../../outside"}); err == nil {
		t.Fatal("unsafe home accepted")
	}
	data, err := os.ReadFile(recordPath(other))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scopedPath(other.SessionID, a), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok := Load(other.SessionID, a); ok {
		t.Fatal("copied record's embedded scope ignored")
	}
}
