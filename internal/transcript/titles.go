package transcript

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const titleIndexLimit = 4 << 20

// SessionTitles returns saved display names keyed by conversation ID. Missing or
// unsupported metadata leaves the caller's existing label fallback intact. Read
// once per radar poll, not once per row; no transcript text is promoted to a title.
func SessionTitles(agent string) map[string]string {
	if normalizeAgent(agent) != "codex" {
		return nil
	}
	f, err := os.Open(filepath.Join(codexHome(), "session_index.jsonl"))
	if err != nil {
		return nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	// The index is append-only: its recent tail contains new sessions and
	// renames. Bound polling cost even when years of sessions have accumulated.
	start := max(int64(0), info.Size()-titleIndexLimit)
	buf := make([]byte, info.Size()-start)
	n, err := f.ReadAt(buf, start)
	if err != nil && err != io.EOF {
		return nil
	}
	buf = buf[:n]
	if start > 0 {
		if i := bytes.IndexByte(buf, '\n'); i >= 0 {
			buf = buf[i+1:]
		} else {
			return nil
		}
	}
	titles := make(map[string]string)
	for line := range bytes.SplitSeq(buf, []byte{'\n'}) {
		var entry struct {
			ID    string `json:"id"`
			Title string `json:"thread_name"`
		}
		if json.Unmarshal(line, &entry) != nil || entry.ID == "" {
			continue
		}
		// The last record for an ID wins, including a cleared name. Collapse
		// whitespace so a saved multiline title cannot break a terminal row.
		titles[entry.ID] = strings.Join(strings.Fields(entry.Title), " ")
	}
	return titles
}
