package agentclient

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// ClaimNative uses the existing durable input protocol for a direct addressed
// transport. The exclusive claim is written BEFORE mutation. A crash therefore
// leaves an uncertain claim, never permission to submit the text again.
func ClaimNative(r Request) error {
	if !instancePattern.MatchString(r.ID) || r.Version != ProtocolVersion {
		return errors.New("invalid native input")
	}
	if _, known := ReadReceipt(r); known {
		return errors.New("native input already settled")
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
	if !instancePattern.MatchString(r.ID) || (outcome != "accepted" && outcome != "uncertain" && outcome != "refused") {
		return errors.New("invalid native input outcome")
	}
	result := Receipt{Version: ProtocolVersion, ID: r.ID, Instance: r.Instance, HomeID: r.HomeID, SessionID: r.SessionID, PID: r.PID, State: outcome, Error: reason, CreatedAt: time.Now().UnixMilli()}
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return atomicPrivate(filepath.Join(InputDir(), r.ID+".receipt.json"), b)
}
