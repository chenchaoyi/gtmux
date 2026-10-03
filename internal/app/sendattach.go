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

	"github.com/chenchaoyi/gtmux/internal/prompt"
	"github.com/chenchaoyi/gtmux/internal/radar"
	"github.com/chenchaoyi/gtmux/internal/tmux"
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
		// made a week ago must not vanish right after this send. Only once the refresh has
		// TAKEN: a copy that keeps its old age is pruned within days, and one the pruner
		// removed between the read and the refresh is gone. Either way it is written anew
		// below, and if that fails too the send fails rather than naming a dead path.
		if reusable(path, len(data)) {
			return path, nil
		}
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

// attachChtimes is os.Chtimes, swappable so a test can make the refresh fail.
var attachChtimes = os.Chtimes

// reusable refreshes an existing copy's age and confirms the result: still a regular file
// of the right size, modified just now.
func reusable(path string, size int) bool {
	now := time.Now()
	if attachChtimes(path, now, now) != nil {
		return false
	}
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular() && fi.Size() == int64(size) &&
		!fi.ModTime().Before(now.Add(-time.Second))
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

// The radar and the screen, swappable for tests.
var (
	holdRadar   = radar.GatherAgents
	holdCapture = tmux.CapturePaneChecked
)

// attachHold says why a message carrying a file must not be typed into a pane now, or ""
// when it may. Such a message is never meant to answer a question, and its text and Enter
// could: pick a permission menu's default, or answer a question in the user's place.
//
// Two reads, neither of them new logic:
//   - The radar's own verdict for the pane, the one every surface shows. It weighs the
//     hook's marker (fresh or stale, a question already answered while the approved tool
//     runs), Codex's approval menu with no marker, and a dispatch stuck at its gate. A
//     marker file on its own is not the verdict: reading it directly refused a pane the
//     radar showed working and let through a menu that had no marker.
//   - The screen, for a menu the hook has not reported yet: prompt.WaitingOptions, the
//     strict detector the radar uses where it has no marker to go on.
//
// A pane that cannot be read is refused: this guard exists to fail closed.
func attachHold(paneID string) string {
	for _, p := range holdRadar() {
		if p.PaneID == paneID {
			if p.Status == "waiting" {
				return "the agent is waiting on a decision (a permission prompt or a question)"
			}
			break
		}
	}
	frame, err := holdCapture(paneID)
	if err != nil {
		return "could not read the pane to check for a question: " + err.Error()
	}
	if prompt.WaitingOptions(frame) != nil {
		return "a choice menu is on the screen"
	}
	return ""
}
