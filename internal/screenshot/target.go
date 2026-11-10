package screenshot

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/chenchaoyi/gtmux/internal/state"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type Target struct {
	Version     int    `json:"schema_version"`
	Pane        string `json:"pane"`
	Loc         string `json:"loc"`
	Agent       string `json:"agent"`
	SessionID   string `json:"session_id"`
	HomeID      string `json:"home_id"`
	Instance    string `json:"instance"`
	ClientPID   int    `json:"client_pid"`
	PanePID     int    `json:"pane_pid"`
	ServerPID   int    `json:"server_pid"`
	ServerStart string `json:"server_start"`
	ExpiresAt   int64  `json:"expires_at"`
}

func targetKey(create bool) ([]byte, error) {
	dir, err := privateDir(create)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "target.key")
	var f *os.File
	if create {
		f, err = state.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	} else {
		err = os.ErrExist
	}
	if err == nil {
		key := make([]byte, 32)
		if _, err = rand.Read(key); err == nil {
			_, err = f.Write(key)
		}
		if err == nil {
			err = f.Sync()
		}
		f.Close()
		if err != nil {
			return nil, err
		}
		return key, nil
	}
	if !os.IsExist(err) {
		return nil, err
	}
	f, err = state.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	i, err := f.Stat()
	if err != nil || !i.Mode().IsRegular() || i.Size() != 32 || i.Mode().Perm()&0077 != 0 {
		return nil, Fail("target-key-unavailable", "Target reference key is not private or complete")
	}
	if s, ok := i.Sys().(*syscall.Stat_t); !ok || s.Uid != uint32(os.Geteuid()) {
		return nil, Fail("target-key-unavailable", "Target reference key has a different owner")
	}
	return io.ReadAll(io.LimitReader(f, 33))
}

func MintTarget(target Target, now time.Time) (string, error) {
	if target.Pane == "" || target.Loc == "" || target.SessionID == "" || target.HomeID == "" || target.Instance == "" || target.ClientPID <= 1 || target.PanePID <= 1 || target.ServerPID <= 1 || target.ServerStart == "" {
		return "", Fail("target-unverified", "An exact live conversation binding is required")
	}
	target.Version, target.ExpiresAt = Version, now.Add(10*time.Minute).Unix()
	key, err := targetKey(true)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(target)
	if err != nil {
		return "", err
	}
	sig := hmac.New(sha256.New, key)
	sig.Write(data)
	return base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sig.Sum(nil)), nil
}

func ParseTarget(token string, now time.Time) (Target, error) {
	var t Target
	if len(token) > 8192 {
		return t, Fail("target-invalid", "Target reference exceeds its budget")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return t, Fail("target-invalid", "Invalid target reference")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return t, Fail("target-invalid", "Invalid target reference")
	}
	got, err := hex.DecodeString(parts[1])
	if err != nil {
		return t, Fail("target-invalid", "Invalid target signature")
	}
	key, err := targetKey(false)
	if err != nil {
		return t, err
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	if !hmac.Equal(got, mac.Sum(nil)) {
		return t, Fail("target-invalid", "Target reference was changed or belongs to another installation")
	}
	if err = Decode(data, &t); err != nil {
		return t, err
	}
	if t.Version != Version || t.ExpiresAt <= now.Unix() || t.ExpiresAt > now.Add(10*time.Minute).Unix() {
		return t, Fail("target-expired", "Resolve the exact target again before a new submission")
	}
	return t, nil
}

func (t Target) SameBinding(current Target) bool {
	t.ExpiresAt, current.ExpiresAt = 0, 0
	t.Version, current.Version = Version, Version
	return t == current
}
