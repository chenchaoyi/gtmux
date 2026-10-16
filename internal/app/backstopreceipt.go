package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Script completion and a changed snapshot are different observations: resurrect
// discards identical layouts. Scope execution evidence to the server and exact
// valid layout so a different server, replaced file or legacy marker cannot pass.
type backstopReceipt struct {
	At        int64           `json:"at"`
	Result    string          `json:"result"`
	Socket    string          `json:"socket,omitempty"`
	ServerPID string          `json:"server_pid,omitempty"`
	Snapshot  saveFingerprint `json:"snapshot"`
}

type saveFingerprint struct {
	Path     string `json:"path"`
	Modified int64  `json:"modified"`
	Digest   string `json:"digest"`
}

func fingerprintSave(path string) (saveFingerprint, bool) {
	real, err := filepath.EvalSymlinks(path)
	if path == "" || err != nil {
		return saveFingerprint{}, false
	}
	f, err := os.Open(real)
	if err != nil {
		return saveFingerprint{}, false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return saveFingerprint{}, false
	}
	// Bound both allocation and work; normal layouts are a few KiB.
	const limit = 4 << 20
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || len(data) > limit {
		return saveFingerprint{}, false
	}
	valid := false
	for _, line := range bytes.Split(data, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("pane\t")) || bytes.HasPrefix(line, []byte("window\t")) {
			valid = true
			break
		}
	}
	if !valid {
		return saveFingerprint{}, false
	}
	sum := sha256.Sum256(data)
	return saveFingerprint{Path: real, Modified: info.ModTime().UnixNano(), Digest: hex.EncodeToString(sum[:])}, true
}

func newBackstopReceipt(at int64, succeeded bool, socket, pid, path string) backstopReceipt {
	r := backstopReceipt{At: at, Result: "failed", Socket: socket, ServerPID: pid}
	if succeeded {
		r.Result = "ok"
		r.Snapshot, _ = fingerprintSave(path)
	}
	return r
}

func readBackstopReceipt() backstopReceipt {
	raw := state.ReadMarker(backstopAttemptPath())
	var r backstopReceipt
	if json.Unmarshal([]byte(raw), &r) == nil {
		return r
	}
	// Preserve retry throttling across an upgrade. An unscoped legacy marker
	// cannot establish health for the current server or snapshot.
	r = backstopReceipt{}
	fields := strings.Fields(raw)
	if len(fields) == 2 {
		r.At, _ = strconv.ParseInt(fields[0], 10, 64)
		r.Result = fields[1]
	}
	return r
}

func writeBackstopReceipt(r backstopReceipt) {
	data, err := json.Marshal(r)
	if err == nil {
		err = state.WriteMarker(backstopAttemptPath(), string(data))
	}
	if err != nil {
		restoreLogf("restore.backstop", "could not persist save receipt: %v", err)
	}
}

func (r backstopReceipt) matches(path, socket, pid string, now time.Time) bool {
	age := now.Sub(time.Unix(r.At, 0))
	if r.Result != "ok" || r.At <= 0 || age < 0 || age >= backstopArmedStaleAfter ||
		socket == "" || pid == "" || r.Socket != socket || r.ServerPID != pid || r.Snapshot.Path == "" {
		return false
	}
	actual, valid := fingerprintSave(path)
	return valid && actual.Modified <= now.UnixNano() && actual == r.Snapshot
}
