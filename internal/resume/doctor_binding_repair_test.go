package resume

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func doctorBindingRepairFixture(t *testing.T, mode string) (CodexFrontend, CodexFrontend) {
	t.Helper()
	old, _ := frontendFixture(t)
	old.SessionID, old.ThreadCwd = "same-thread", old.Cwd
	now := time.Now()
	old.StartedAt = now.Add(-time.Minute).UnixMilli()
	old.UpdatedAt = now.Add(-10 * time.Second).UnixMilli()
	current := old
	current.Instance, current.Sequence = strings.Repeat("a", 32), old.Sequence+1
	current.StartedAt, current.UpdatedAt = now.Add(-time.Second).UnixMilli(), now.UnixMilli()
	switch mode {
	case "other-thread":
		current.SessionID = "other-thread"
	case "other-terminal":
		current.TTY = "ttys100"
	case "other-home":
		current.HomeID = strings.Repeat("b", 32)
	case "future-receipt":
		current.UpdatedAt = now.Add(time.Minute).UnixMilli()
	}
	if err := os.MkdirAll(filepath.Dir(frontendPath(old.Instance)), 0700); err != nil {
		t.Fatal(err)
	}
	for _, f := range []CodexFrontend{old, current} {
		if err := writeBindingJSON(frontendPath(f.Instance), f); err != nil {
			t.Fatal(err)
		}
	}
	if mode == "ambiguous" {
		other := current
		other.Instance = strings.Repeat("c", 32)
		if err := writeBindingJSON(frontendPath(other.Instance), other); err != nil {
			t.Fatal(err)
		}
	}
	if mode != "no-committed-binding" {
		recordCrossPaneBinding(t, current)
	}
	index := frontendRecord(old)
	index.RecoverySessionID, index.RecoveryCwd, index.RecoveryAt = old.SessionID, old.ThreadCwd, 123
	raw, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw[:len(raw)-1], []byte(`,"future_field":{"retained":true}}`)...)
	if err := os.WriteFile(fileFor(old.Loc), raw, 0600); err != nil {
		t.Fatal(err)
	}
	return old, current
}

func TestDBR001DoctorRepairsOnlyTheVerifiedReference(t *testing.T) {
	old, live := doctorBindingRepairFixture(t, "")
	path := fileFor(old.Loc)
	before := indexBytes(t, path)
	oldBytes := indexBytes(t, frontendPath(old.Instance))
	if _, ok := LoadLive(old.Loc); ok {
		t.Fatal("old attachment was already usable")
	}
	plan, ok := PlanConversationBindingRepair(old.Loc, old.Pane)
	if !ok || !bytes.Equal(before, indexBytes(t, path)) {
		t.Fatal("planning failed or wrote the index")
	}
	live.UpdatedAt++ // A heartbeat after the prompt may belong to the same connection.
	if err := writeBindingJSON(frontendPath(live.Instance), live); err != nil {
		t.Fatal(err)
	}
	liveBytes := indexBytes(t, frontendPath(live.Instance))
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	r, ok := LoadLive(old.Loc)
	if !ok || r.SessionID != old.SessionID || r.Frontend != live.Instance || r.RecoveryAt != 123 {
		t.Fatal("repaired live link or recovery checkpoint differs", r, ok)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(indexBytes(t, path), &fields); err != nil || string(fields["future_field"]) != `{"retained":true}` {
		t.Fatalf("unknown index fields changed: %v %v", fields, err)
	}
	if !bytes.Equal(oldBytes, indexBytes(t, frontendPath(old.Instance))) ||
		!bytes.Equal(liveBytes, indexBytes(t, frontendPath(live.Instance))) {
		t.Fatal("a producer receipt was changed")
	}
	if _, ok := PlanConversationBindingRepair(old.Loc, old.Pane); ok {
		t.Fatal("a repaired index was proposed again")
	}
}

func TestDBR001DoctorRefusesIncompleteOrChangedEvidence(t *testing.T) {
	for _, mode := range []string{"other-thread", "other-terminal", "other-home", "future-receipt", "ambiguous", "no-committed-binding"} {
		t.Run(mode, func(t *testing.T) {
			old, _ := doctorBindingRepairFixture(t, mode)
			before := indexBytes(t, fileFor(old.Loc))
			if _, ok := PlanConversationBindingRepair(old.Loc, old.Pane); ok || !bytes.Equal(before, indexBytes(t, fileFor(old.Loc))) {
				t.Fatal("unverified owner was offered or written")
			}
		})
	}
	t.Run("lease-and-index-change", func(t *testing.T) {
		old, _ := doctorBindingRepairFixture(t, "")
		if _, ok := PlanConversationBindingRepair(old.Loc, "%88"); ok {
			t.Fatal("another pane was authorized")
		}
		plan, ok := PlanConversationBindingRepair(old.Loc, old.Pane)
		if !ok {
			t.Fatal("fixture has no verified repair")
		}
		lease, err := state.TryLease("codex-frontend-binding")
		if err != nil {
			t.Fatal(err)
		}
		err = plan.Apply()
		lease.Close()
		if !errors.Is(err, state.ErrLeaseHeld) {
			t.Fatalf("repair bypassed the producer lease: %v", err)
		}
		path := fileFor(old.Loc)
		other := append(indexBytes(t, path), '\n')
		if err := os.WriteFile(path, other, 0600); err != nil {
			t.Fatal(err)
		}
		if err := plan.Apply(); err == nil || !bytes.Equal(other, indexBytes(t, path)) {
			t.Fatal("repair overwrote intervening index bytes", err)
		}
	})
	t.Run("binding-replaced-after-preview", func(t *testing.T) {
		old, live := doctorBindingRepairFixture(t, "")
		plan, ok := PlanConversationBindingRepair(old.Loc, old.Pane)
		if !ok {
			t.Fatal("fixture has no verified repair")
		}
		before := indexBytes(t, fileFor(old.Loc))
		other := live
		other.Pane, other.Loc = "%88", "other:0.0"
		recordCrossPaneBinding(t, other)
		binding, ok := native.BindingFor(live.SessionID, live.HomeID)
		if !ok || binding.Pane != other.Pane {
			t.Fatal("fixture did not replace the native binding")
		}
		if err := plan.Apply(); err == nil || !bytes.Equal(before, indexBytes(t, fileFor(old.Loc))) {
			t.Fatal("repair overwrote a superseded owner", err)
		}
	})
}
