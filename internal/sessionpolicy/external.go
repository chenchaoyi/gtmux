package sessionpolicy

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

// Unknown native control is not a grant. The durable metadata fence outlives
// native SessionEnd so mining cannot silently fall back to creation originator.
type externalFence struct {
	ID    string `json:"session_id"`
	Since int64  `json:"since"`
}

func fencePath(id string) string { return path(id) + ".external" }

func RequiresConsent(id, client string) bool {
	if client == "unconfirmed" || client == "chatgpt_desktop" || Desktop(id) {
		return true
	}
	b, err := os.ReadFile(fencePath(id))
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return true
	}
	var v externalFence
	if err := json.Unmarshal(b, &v); err != nil || v.ID != id || v.Since <= 0 {
		diag.For("session-follow").Warn("external.fence.invalid", "invalid external observation boundary; requiring consent", "agent_session", id)
	}
	return true
}

// MarkExternal is called only for actual native turn activity, not readers or
// provisional SessionStart. An existing fence is immutable and permission-free.
func MarkExternal(id string, now int64) error {
	if id == "" || len(id) > 256 || now <= 0 {
		return ErrUnverified
	}
	file := fencePath(id)
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return err
	}
	b, err := os.ReadFile(file)
	if err == nil {
		var v externalFence
		if json.Unmarshal(b, &v) != nil || v.ID != id || v.Since <= 0 {
			return errors.New("invalid external observation fence")
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	b, _ = json.Marshal(externalFence{ID: id, Since: now})
	// O_EXCL selects the first observation; subsequent writers never reset consent.
	f, err := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		diag.For("session-follow").Warn("external.fence.failed", "could not persist external observation boundary", "agent_session", id, "error", err)
		return err
	}
	return closeErr
}
