package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// maxAttachBytes matches the phone's upload ceiling (POST /api/upload).
const maxAttachBytes = 30 << 20

// attachFiles copies each file into the uploads dir and returns the absolute paths an
// agent can read, in order. `gtmux send --attach` hands a screenshot or a file to an
// agent the way the phone does: the message, then one path per line.
//
// The copy is named by its CONTENT (a hash prefix), not a random token like the phone's
// uploads. A retry of the same image must produce the same message, or the send
// interlock — which refuses an identical payload sent twice — cannot recognise it, and a
// menu-bar retry after an unconfirmed delivery would land a second copy.
func attachFiles(paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !fi.Mode().IsRegular() {
			return nil, fmt.Errorf("%s: not a regular file", p)
		}
		if fi.Size() > maxAttachBytes {
			return nil, fmt.Errorf("%s: larger than %d MB", p, maxAttachBytes>>20)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		dst, err := saveAttachment(filepath.Base(p), data)
		if err != nil {
			return nil, err
		}
		out = append(out, dst)
	}
	return out, nil
}

// saveAttachment writes data under the uploads dir as <hash>-<safe name>. The same bytes
// under the same name land on the same path, and an existing identical file is reused.
func saveAttachment(name string, data []byte) (string, error) {
	dir := uploadsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	safe := sanitizeFilename(name)
	if safe == "" {
		safe = "attachment"
	}
	sum := sha256.Sum256(data)
	path := filepath.Join(dir, hex.EncodeToString(sum[:])[:12]+"-"+safe)
	if have, err := os.ReadFile(path); err == nil && bytes.Equal(have, data) {
		// Reused, so it counts as new: the uploads dir is pruned by age, and a copy first
		// made a week ago must not vanish right after this send.
		now := time.Now()
		_ = os.Chtimes(path, now, now)
		return path, nil
	}
	tmp, err := os.CreateTemp(dir, ".attach-*")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return path, nil
}

// withAttachments appends one path per line after the message, as the phone's composer
// does, so an agent reads the note first and finds each file on a line of its own.
func withAttachments(text string, paths []string) string {
	if len(paths) == 0 {
		return text
	}
	list := strings.Join(paths, "\n")
	if strings.TrimSpace(text) == "" {
		return list
	}
	return strings.TrimRight(text, "\n") + "\n" + list
}

// stateRefusedWaiting is send's verdict for a message with an attachment aimed at an agent
// that is waiting on the user.
const stateRefusedWaiting = "refused-waiting"

// attachWaiting reports whether the pane's agent is waiting on the user: the hook's
// waiting marker for that pane exists.
func attachWaiting(paneID string) bool {
	if paneID == "" {
		return false
	}
	_, err := os.Stat(state.WaitingPath(paneID))
	return err == nil
}
