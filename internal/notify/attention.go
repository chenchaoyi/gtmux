package notify

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Attention snapshots let the desktop withdraw managed notices even after a
// restart or when the ordinary, short-lived posting queue has expired.
const attentionLimit = 128 << 10

func SyncAttention(scope string, active []string) error {
	switch scope {
	case "workspace", "relay":
	default:
		return errors.New("unknown desktop attention scope")
	}
	keys := append([]string{}, active...)
	for _, key := range keys {
		if !strings.HasPrefix(key, scope+"/") || len(key) <= len(scope)+1 {
			return errors.New("invalid desktop attention identity")
		}
	}
	slices.Sort(keys)
	data, err := json.Marshal(slices.Compact(keys))
	if err != nil {
		return err
	}
	if len(data) > attentionLimit {
		return errors.New("desktop attention snapshot exceeds the size limit")
	}
	path := filepath.Join(state.NotifyDir(), "hq-"+scope+".attention")
	f, err := state.OpenPrivateRead(path)
	if err == nil {
		previous, readErr := io.ReadAll(io.LimitReader(f, attentionLimit+1))
		f.Close()
		if readErr != nil {
			return readErr
		}
		if bytes.Equal(previous, data) {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return state.WritePrivateFile(path, data)
}
