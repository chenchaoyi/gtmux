package transcript

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/klauspost/compress/zstd"
)

// RecoveryInfo describes durable history, never a live attachment or input lease.
// A paginated thread still has a canonical session_meta rollout; no mutable SQLite
// schema or completed model turn is required to recognize that recovery point.
type RecoveryInfo struct {
	State string
	Cwd   string
}

const (
	RecoveryReady      = "ready"
	RecoveryMissing    = "missing"
	RecoveryUnreadable = "unreadable"
	RecoveryInvalid    = "invalid"
)

func codexRolloutName(name, id string) (base, continuation bool) {
	name = strings.TrimSuffix(name, ".zst")
	base = strings.HasPrefix(name, "rollout-") && strings.HasSuffix(name, "-"+id+".jsonl")
	continuation = strings.HasPrefix(name, "rollout-") && strings.HasSuffix(name, ".jsonl") &&
		strings.Contains(strings.TrimSuffix(name, ".jsonl"), "-"+id+"_") && !base
	return
}

// CodexRecovery checks the same home and rollout identities used by Chat, including
// archived and continued rollouts. Only bounded metadata headers are inspected.
// Missing, denied and malformed history remain distinct; none authorize a resume.
func CodexRecovery(id string) RecoveryInfo {
	result := RecoveryInfo{State: RecoveryMissing}
	if id == "" || len(id) > 256 || strings.ContainsAny(id, "*/?[]\\/") || strings.IndexFunc(id, unicode.IsControl) >= 0 {
		result.State = RecoveryInvalid
		return result
	}
	for _, dir := range []string{"sessions", "archived_sessions"} {
		root := filepath.Join(codexHome(), dir)
		resolved, err := filepath.EvalSymlinks(root)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			result.State = RecoveryUnreadable
			continue
		}
		err = filepath.WalkDir(resolved, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				if result.State != RecoveryReady {
					result.State = RecoveryUnreadable
				}
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			base, continued := codexRolloutName(entry.Name(), id)
			if !base && !continued {
				return nil
			}
			// Codex prefers a plain sibling during compression/materialization.
			// A stale compressed copy must not validate an invalid current file.
			if strings.HasSuffix(path, ".zst") {
				if _, err := os.Lstat(strings.TrimSuffix(path, ".zst")); err == nil {
					return nil
				} else if !errors.Is(err, fs.ErrNotExist) {
					result.State = RecoveryUnreadable
					return nil
				}
			}
			info := inspectCodexRecoveryFile(path, id)
			if info.State == RecoveryReady {
				result = info
				return fs.SkipAll
			}
			if result.State != RecoveryUnreadable || info.State == RecoveryUnreadable {
				result = info
			}
			return nil
		})
		if result.State == RecoveryReady {
			return result
		}
		if err != nil {
			result.State = RecoveryUnreadable
		}
	}
	return result
}

func inspectCodexRecoveryFile(path, id string) RecoveryInfo {
	bad := RecoveryInfo{State: RecoveryInvalid}
	info, err := os.Lstat(path)
	if err != nil {
		return RecoveryInfo{State: RecoveryUnreadable}
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return bad
	}
	f, err := os.Open(path)
	if err != nil {
		return RecoveryInfo{State: RecoveryUnreadable}
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return RecoveryInfo{State: RecoveryUnreadable}
	}
	var reader io.Reader = f
	if strings.HasSuffix(path, ".jsonl.zst") {
		// Inspect the decoded header without materializing or rewriting history.
		// Bound the stream decoder window as well as the metadata line size.
		decoder, err := zstd.NewReader(f, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(64<<20))
		if err != nil {
			return bad
		}
		defer decoder.Close()
		reader = decoder
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	if !scanner.Scan() {
		return bad
	}
	var header struct {
		Type    string `json:"type"`
		Payload struct {
			ID        string `json:"id"`
			SessionID string `json:"session_id"`
			Cwd       string `json:"cwd"`
		} `json:"payload"`
	}
	if json.Unmarshal(scanner.Bytes(), &header) != nil || header.Type != "session_meta" {
		return bad
	}
	actual := header.Payload.ID
	if actual == "" {
		actual = header.Payload.SessionID
	}
	if actual != id {
		return bad
	}
	if header.Payload.Cwd != "" && (!filepath.IsAbs(header.Payload.Cwd) || strings.IndexFunc(header.Payload.Cwd, unicode.IsControl) >= 0) {
		return bad
	}
	return RecoveryInfo{State: RecoveryReady, Cwd: header.Payload.Cwd}
}
