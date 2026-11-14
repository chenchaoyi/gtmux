package hqinput

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// ReadRotationFile shares the bounded read-only private-file boundary with the
// resident maintenance reader. It never follows or blocks on a special leaf.
func ReadRotationFile() ([]byte, error) {
	f, err := state.OpenPrivateRead(filepath.Join(state.Dir(), "hqwake", "rotate-pending.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil {
		return nil, err
	}
	if len(b) > 8192 {
		return nil, errors.New("HQ rotation reservation exceeds read budget")
	}
	return b, nil
}

// RotationPending reads an input reservation, never authority to reset a session.
// Callers hold Acquire to serialize this observation with request registration.
func RotationPending(pane string) (bool, error) {
	b, err := ReadRotationFile()
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var r struct {
		Pane, Agent, Retiring, Input string
		CreatedAt, SentAt            int64
	}
	if json.Unmarshal(b, &r) != nil || r.Pane == "" || r.Agent == "" || r.Retiring == "" || r.Input == "" || r.CreatedAt <= 0 || r.SentAt < 0 {
		return false, errors.New("HQ rotation reservation is unreadable")
	}
	return r.Pane == pane && r.SentAt == 0, nil
}
