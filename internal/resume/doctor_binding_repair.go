package resume

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

// ConversationBindingRepair only corrects a reference to an independently
// committed frontend. The bridge remains the only producer of live proof.
type ConversationBindingRepair struct {
	raw      []byte
	old, now CodexFrontend
}

func PlanConversationBindingRepair(loc, pane string) (ConversationBindingRepair, bool) {
	var plan ConversationBindingRepair
	var index Record
	raw, err := readCrossPaneIndex(fileFor(loc))
	if err != nil || json.Unmarshal(raw, &index) != nil || index.Agent != "codex" ||
		index.Frontend == "" || index.FrontendRetired || index.TerminalClient != "" ||
		index.HomeID != transcript.CodexHomeID() {
		return plan, false
	}
	old, ok := readStoredFrontend(index.Frontend)
	if !ok || old.Retired || old.Pane != pane || old.Loc != loc || old.SessionID != index.SessionID ||
		old.HomeID != index.HomeID || time.Now().UnixMilli()-old.UpdatedAt <= 5000 {
		return plan, false
	}
	live, ok := conversationFrontend(loc)
	if !ok || live.CodexBindingTarget != old.CodexBindingTarget || live.HomeID != index.HomeID ||
		live.SessionID != index.SessionID || live.ThreadCwd != index.Cwd ||
		live.BridgePID != old.BridgePID || live.BridgeStart != old.BridgeStart || live.Sequence <= old.Sequence {
		return plan, false
	}
	committed, ok := native.BindingFor(live.SessionID, live.HomeID)
	if !ok || committed.Kind != "frontend" || committed.ID != live.Instance || committed.Loc != loc ||
		committed.Pane != pane || committed.PID != live.PID || committed.PanePID != live.PanePID ||
		committed.BackendPID != live.BackendPID || committed.BackendStart != live.BackendStart || committed.Cwd != live.ThreadCwd {
		return plan, false
	}
	return ConversationBindingRepair{raw: raw, old: old, now: live}, true
}

func (repair ConversationBindingRepair) Apply() error {
	lease, err := state.TryLease("codex-frontend-binding")
	if err != nil {
		return err
	}
	defer lease.Close()
	current, ok := PlanConversationBindingRepair(repair.old.Loc, repair.old.Pane)
	expected := repair.now
	// The same connection may have sent another heartbeat during confirmation.
	expected.UpdatedAt = current.now.UpdatedAt
	if !ok || !bytes.Equal(current.raw, repair.raw) || current.old != repair.old ||
		current.now != expected || current.now.UpdatedAt < repair.now.UpdatedAt {
		return errors.New("conversation binding evidence changed")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(current.raw, &fields); err != nil {
		return err
	}
	fields["codex_frontend"], _ = json.Marshal(current.now.Instance)
	if err := verifyResumeIndexBytes(current.old.Loc, current.raw); err != nil {
		return err
	}
	if err := frontendBindingWrite(fileFor(current.old.Loc), fields); err != nil {
		return err
	}
	readback, live := LoadLive(current.old.Loc)
	if !live || readback.SessionID != current.now.SessionID || readback.HomeID != current.now.HomeID || readback.Frontend != current.now.Instance {
		return errors.New("conversation binding could not be verified after saving")
	}
	return nil
}
