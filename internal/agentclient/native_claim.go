package agentclient

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// ClaimNative uses the existing durable input protocol for a direct addressed
// transport. The exclusive claim is written BEFORE mutation. A crash therefore
// leaves an uncertain claim, never permission to submit the text again.
func ClaimNative(r Request) error {
	if !instancePattern.MatchString(r.ID) || r.Version != ProtocolVersion {
		return errors.New("invalid native input")
	}
	lease, err := state.TryLease("terminal-input")
	if err != nil {
		return err
	}
	defer lease.Close()
	if _, known := ReadReceipt(r); known {
		return errors.New("native input already settled")
	}
	var pending Request
	if readOwnedJSON(filepath.Join(InputDir(), r.ID+".pending.json"), &pending, 2<<20) != nil {
		return errors.New("native input is no longer pending")
	}
	// Creation time drives retention, not authority. Every actual address and
	// payload field must still match the original owned pending request.
	pending.CreatedAt = r.CreatedAt
	if pending != r {
		return errors.New("native pending input changed")
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := writeExclusive(filepath.Join(InputDir(), r.ID+".claimed.json"), b); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(InputDir(), r.ID+".pending.json"))
	return nil
}

func FinishNative(r Request, outcome, reason string) error {
	return FinishNativeDelivery(r, outcome, reason, "")
}

// Delivery is optional acceptance metadata, not proof of model execution.
func FinishNativeDelivery(r Request, outcome, reason, delivery string) error {
	if !instancePattern.MatchString(r.ID) || (outcome != "accepted" && outcome != "uncertain" && outcome != "refused") {
		return errors.New("invalid native input outcome")
	}
	if delivery != "" && (outcome != "accepted" || (delivery != "active-turn" && delivery != "started-turn")) {
		return errors.New("invalid native delivery metadata")
	}
	result := Receipt{Version: ProtocolVersion, ID: r.ID, Instance: r.Instance, HomeID: r.HomeID, SessionID: r.SessionID, PID: r.PID, State: outcome, Delivery: delivery, Error: reason, CreatedAt: time.Now().UnixMilli()}
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return atomicPrivate(filepath.Join(InputDir(), r.ID+".receipt.json"), b)
}
