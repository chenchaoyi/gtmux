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

func attentionPath(scope string) (string, error) {
	switch scope {
	case "workspace", "relay":
		return filepath.Join(state.NotifyDir(), "hq-"+scope+".attention"), nil
	}
	return "", errors.New("unknown desktop attention scope")
}

func readAttention(path string) ([]byte, error) {
	f, err := state.OpenPrivateRead(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, attentionLimit+1))
	if len(data) > attentionLimit {
		return nil, errors.New("desktop attention snapshot exceeds the size limit")
	}
	return data, err
}

func HasActiveAttention(scope string) (bool, error) {
	path, err := attentionPath(scope)
	if err != nil {
		return false, err
	}
	data, err := readAttention(path)
	if err != nil || data == nil {
		return false, err
	}
	var keys []string
	if err := json.Unmarshal(data, &keys); err != nil {
		return false, err
	}
	if keys == nil {
		return false, errors.New("invalid desktop attention snapshot")
	}
	return len(keys) > 0, nil
}

func SyncAttention(scope string, active []string) error {
	path, err := attentionPath(scope)
	if err != nil {
		return err
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
	previous, err := readAttention(path)
	if err != nil || bytes.Equal(previous, data) {
		return err
	}
	return state.WritePrivateFile(path, data)
}
