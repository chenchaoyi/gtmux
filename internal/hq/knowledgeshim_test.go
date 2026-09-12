package hq

import (
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/knowledge"
)

// The doctor verdict over the queue: empty quiet, young OK, stale flagged,
// landed back to quiet.
func TestPromotionsStatus(t *testing.T) {
	asHQ(t)
	now := int64(2_000_000_000)
	if r := PromotionsStatus(now); r.Pending != 0 || r.State != MaintenanceOK {
		t.Fatalf("empty queue must be quiet: %+v", r)
	}
	if rc := knowledge.CmdKnowledge([]string{"add", "--topic", "workflows", "--title", "release flow"}); rc != 0 {
		t.Fatal("add failed")
	}
	id := "workflows/" + knowledge.Slug("release flow")
	if rc := knowledge.CmdKnowledge([]string{"promote", id, "--why", "w"}); rc != 0 {
		t.Fatal("promote failed")
	}
	promotedAt := time.Now().Unix()
	if r := PromotionsStatus(promotedAt + 3600); r.Pending != 1 || r.State != MaintenanceOK {
		t.Fatalf("a young pending promotion is OK with figures: %+v", r)
	}
	if r := PromotionsStatus(promotedAt + promotionStaleSecs + 1); r.State != MaintenanceSlipped {
		t.Fatalf("a stale pending promotion must flag: %+v", r)
	}
	if rc := knowledge.CmdKnowledge([]string{"land", id, "--ref", "#1"}); rc != 0 {
		t.Fatal("land failed")
	}
	if r := PromotionsStatus(promotedAt + promotionStaleSecs + 2); r.Pending != 0 || r.State != MaintenanceOK {
		t.Fatalf("landing must clear the row: %+v", r)
	}
}
