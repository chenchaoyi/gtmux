package screenshot

import (
	"github.com/chenchaoyi/gtmux/internal/state"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestASC004TargetReferencesBindWholeIncarnationAndExpire(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Unix(10000, 0)
	v := Target{Pane: "%9", Loc: "synthetic:0.0", Agent: "codex", SessionID: "s", HomeID: "h", Instance: "i", ClientPID: 100, PanePID: 99, ServerPID: 90, ServerStart: "birth"}
	token, e := MintTarget(v, now)
	if e != nil {
		t.Fatal(e)
	}
	got, e := ParseTarget(token, now)
	if e != nil || !got.SameBinding(v) {
		t.Fatal(got, e)
	}
	for _, change := range []func(*Target){func(x *Target) { x.SessionID = "other" }, func(x *Target) { x.HomeID = "other" }, func(x *Target) { x.PanePID++ }, func(x *Target) { x.ServerStart = "new" }, func(x *Target) { x.ClientPID++ }, func(x *Target) { x.Instance = "new" }, func(x *Target) { x.Agent = "dsh" }} {
		other := v
		change(&other)
		if got.SameBinding(other) {
			t.Fatal("retargeted", other)
		}
	}
	if _, e = ParseTarget(token, now.Add(10*time.Minute)); e == nil {
		t.Fatal("expired")
	}
	parts := strings.Split(token, ".")
	if _, e = ParseTarget(parts[0]+"."+strings.Repeat("0", 64), now); e == nil {
		t.Fatal("tampered")
	}
	if _, e = MintTarget(Target{}, now); e == nil {
		t.Fatal("missing identity accepted")
	}
}
func TestASC004TargetQueriesDoNotMintKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, _ = ParseTarget("e30."+strings.Repeat("0", 64), time.Now())
	if _, e := os.Stat(filepath.Join(state.Dir(), "capture-requests")); !os.IsNotExist(e) {
		t.Fatal("parse created key state")
	}
}
