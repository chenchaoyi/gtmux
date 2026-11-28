package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"
)

const codexWitnessBudget int64 = 256 << 10

// CodexFirstTurnStart reads the first persisted start from one original
// rollout. Callers still need live identity and their own time window.
func CodexFirstTurnStart(session string) time.Time {
	paths := matchingCodexLogPaths(session, enumerateCodexCandidates())
	if !unambiguousCodexOriginal(session, paths) {
		return time.Time{}
	}
	f, err := os.Open(paths[0])
	if err != nil {
		return time.Time{}
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return time.Time{}
	}
	meta, headerEnd := readCodexStartupHeader(f, info.Size())
	id := meta.ID
	if id == "" {
		id = meta.SessionID
	}
	if headerEnd == 0 || id != session || (meta.SessionID != "" && meta.SessionID != session) {
		return time.Time{}
	}
	rows := readCodexStartupRows(f, info.Size(), headerEnd)
	for len(rows) > 0 {
		row, rest, complete := bytes.Cut(rows, []byte{'\n'})
		var line codexLine
		if !complete || json.Unmarshal(row, &line) != nil || line.Type == "" {
			return time.Time{}
		}
		rows = rest
		if line.Type != "event_msg" {
			continue
		}
		var event struct {
			Type string `json:"type"`
			Turn string `json:"turn_id"`
		}
		if json.Unmarshal(line.Payload, &event) != nil || event.Type == "" {
			return time.Time{}
		}
		switch event.Type {
		case "task_complete", "turn_aborted":
			return time.Time{}
		case "task_started":
			if event.Turn != "" {
				if at, err := time.Parse(time.RFC3339Nano, line.Timestamp); err == nil {
					return at
				}
			}
			return time.Time{}
		}
	}
	return time.Time{}
}

// Chat folds matching basenames for display. A startup prefix needs one
// original rollout, so same-name paths may coexist only as aliases of one file.
func unambiguousCodexOriginal(session string, paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	name := filepath.Base(paths[0])
	if base, _ := codexRolloutName(name, session); !base {
		return false
	}
	if len(paths) == 1 {
		return true
	}
	first, err := os.Stat(paths[0])
	if err != nil || !first.Mode().IsRegular() {
		return false
	}
	for _, path := range paths[1:] {
		if filepath.Base(path) != name {
			return false
		}
		info, err := os.Stat(path)
		if err != nil || !os.SameFile(first, info) {
			return false
		}
	}
	return true
}

func readCodexStartupHeader(f *os.File, size int64) (codexPayload, int64) {
	header := bufio.NewReader(io.NewSectionReader(f, 0, min(size, codexSessionMetaBudget)))
	row, err := header.ReadBytes('\n')
	if err != nil {
		return codexPayload{}, 0
	}
	return parseCodexSessionMeta(row), int64(len(row))
}

func readCodexStartupRows(f *os.File, size, headerEnd int64) []byte {
	prefix := make([]byte, min(codexWitnessBudget, size-headerEnd))
	if _, err := f.ReadAt(prefix, headerEnd); err != nil {
		return nil
	}
	return prefix
}
