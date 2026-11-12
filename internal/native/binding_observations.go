package native

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// PrivateBindingObservations reads independent historical receipts, not live
// attachments. It neither prunes observations nor grants input/restore authority.
func PrivateBindingObservations(home string) ([]Binding, error) {
	if home == "" || !validHomeID(home) {
		return nil, errors.New("unknown binding home")
	}
	dir := filepath.Join(bindingsDir(), home)
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 || !observationOwner(info) {
		return nil, errors.New("unsafe binding directory")
	}
	d, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	entries, err := d.ReadDir(513)
	_ = d.Close()
	if err != nil && err != io.EOF || len(entries) > 512 {
		return nil, errors.New("binding observation bound exceeded")
	}
	var out []Binding
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		before, err := os.Lstat(path)
		if err != nil || !before.Mode().IsRegular() || before.Mode().Perm()&0077 != 0 || before.Size() > 8192 || !observationOwner(before) {
			return nil, errors.New("unsafe binding observation")
		}
		f, err := state.OpenPrivateRead(path)
		if err != nil {
			return nil, err
		}
		opened, statErr := f.Stat()
		data, readErr := io.ReadAll(io.LimitReader(f, 8193))
		_ = f.Close()
		if statErr != nil || !os.SameFile(before, opened) || readErr != nil || len(data) > 8192 {
			return nil, errors.New("binding observation changed")
		}
		var b Binding
		if json.Unmarshal(data, &b) != nil || b.HomeID != home || b.SessionID == "" || len(b.SessionID) > 256 ||
			strings.ContainsAny(b.SessionID, "\r\n\x00") || entry.Name() != base64.RawURLEncoding.EncodeToString([]byte(b.SessionID))+".json" ||
			b.ID == "" || b.Pane == "" || b.Loc == "" || b.PID <= 1 || b.PanePID <= 1 || b.PreparedAt <= 0 || b.BoundAt < b.PreparedAt {
			return nil, errors.New("invalid binding observation")
		}
		out = append(out, b)
	}
	return out, nil
}

func observationOwner(info os.FileInfo) bool {
	owner, ok := info.Sys().(*syscall.Stat_t)
	return ok && owner.Uid == uint32(os.Geteuid())
}
