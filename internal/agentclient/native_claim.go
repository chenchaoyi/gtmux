package agentclient

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// ClaimNative uses the existing durable input protocol for a direct addressed
// transport. The exclusive claim is written BEFORE mutation. A crash therefore
// leaves an uncertain claim, never permission to submit the text again.
func ClaimNative(r Request) error {
	return claimNative(r, syncInput)
}

func claimNative(r Request, ops inputDurability) error {
	if !instancePattern.MatchString(r.ID) || r.Version != ProtocolVersion {
		return errors.New("invalid native input")
	}
	if err := r.CheckInputStore(); err != nil {
		return err
	}
	lease, err := state.TryLease("terminal-input")
	if err != nil {
		return err
	}
	defer lease.Close()
	if err := r.CheckInputStore(); err != nil {
		return err
	}
	queue, err := openInputQueue(r.store, ops)
	if err != nil {
		return err
	}
	defer queue.dir.Close()
	pendingName, claimName, receiptName := r.ID+".pending.json", r.ID+".claimed.json", r.ID+".receipt.json"
	if !queue.missing(receiptName) {
		return ErrInputUncertain
	}
	var pending Request
	if data, err := queue.read(pendingName, 2<<20); err != nil ||
		decodeInputRecord(data, &pending, requestFields) != nil || pending.CreatedAt <= 0 {
		return errors.New("native input is no longer pending")
	}
	// Creation time drives retention, not authority. Every actual address and
	// payload field must still match the original owned pending request.
	pending.CreatedAt = r.CreatedAt
	if pending != r.persisted() {
		return errors.New("native pending input changed")
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := queue.createSynced(claimName, b); err != nil {
		return err
	}
	if err := queue.syncBoundIndex(r); err != nil {
		return errors.Join(ErrInputUncertain, err)
	}
	if !queue.missing(receiptName) {
		return ErrInputUncertain
	}
	if err := queue.remove(pendingName); err != nil {
		return err
	}
	return r.CheckInputStore()
}

func FinishNative(r Request, outcome, reason string) error {
	return FinishNativeDelivery(r, outcome, reason, "")
}

// Delivery is optional acceptance metadata, not proof of model execution.
func FinishNativeDelivery(r Request, outcome, reason, delivery string) error {
	return finishNativeDelivery(r, outcome, reason, delivery, syncInput)
}

func finishNativeDelivery(r Request, outcome, reason, delivery string, ops inputDurability) error {
	if !instancePattern.MatchString(r.ID) || (outcome != "accepted" && outcome != "uncertain" && outcome != "refused") {
		return errors.New("invalid native input outcome")
	}
	if delivery != "" && (outcome != "accepted" || (delivery != "active-turn" && delivery != "started-turn" && delivery != "wake-accepted")) {
		return errors.New("invalid native delivery metadata")
	}
	// An outcome may arrive while admission/pruning holds the lease. Allow
	// brief contention to clear before reporting a missing acknowledgement.
	deadline := time.Now().Add(2 * time.Second)
	if err := r.CheckInputStore(); err != nil {
		return err
	}
	lease, err := state.TryLease("terminal-input")
	for errors.Is(err, state.ErrLeaseHeld) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		lease, err = state.TryLease("terminal-input")
	}
	if err != nil {
		return err
	}
	defer lease.Close()
	if err := r.CheckInputStore(); err != nil {
		return err
	}
	queue, err := openInputQueue(r.store, ops)
	if err != nil {
		return err
	}
	defer queue.dir.Close()
	if old, err := queue.committedReceipt(r); err == nil {
		if old.State != outcome || old.Error != reason || old.Delivery != delivery {
			return errors.New("native input has a different recorded outcome")
		}
	} else {
		if errors.Is(err, ErrInputUncertain) || !errors.Is(err, os.ErrNotExist) {
			return ErrInputUncertain
		}
		if !queue.hasNativeClaim(r) {
			return errors.New("native input claim does not match")
		}
		result := Receipt{Version: ProtocolVersion, ID: r.ID, Instance: r.Instance, HomeID: r.HomeID, SessionID: r.SessionID, PID: r.PID, State: outcome, Delivery: delivery, Error: reason, CreatedAt: time.Now().UnixMilli()}
		b, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if len(b) > 8192 {
			return errors.New("native receipt exceeds its read limit")
		}
		if err := queue.publishDocument(r.ID+".receipt.json", b); err != nil {
			return err
		}
	}
	if err := r.CheckInputStore(); err != nil {
		return err
	}
	// A committed receipt remains authoritative if compaction must wait.
	if outcome != "uncertain" && queue.missing(r.ID+".pending.json") && queue.hasNativeClaim(r) {
		_ = queue.remove(r.ID + ".claimed.json")
	}
	return nil
}

func (q *inputQueue) hasNativeClaim(r Request) bool {
	data, err := q.read(r.ID+".claimed.json", 2<<20)
	var claim Request
	return err == nil && decodeInputRecord(data, &claim, requestFields) == nil && claim == r.persisted()
}
