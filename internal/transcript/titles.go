package transcript

import (
	"bytes"
	"encoding/json"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const titleIndexLimit = 4 << 20

var (
	codexTitlesMu    sync.Mutex
	codexTitlesState struct {
		path      string
		ino       uint64
		size      int64
		mtimeNano int64
		titles    map[string]string
	}
)

// SessionTitles returns saved display names keyed by conversation ID. Missing or
// unsupported metadata leaves the caller's existing label fallback intact. Read
// once per radar poll, not once per row; no transcript text is promoted to a title.
func SessionTitles(agent string) map[string]string {
	if normalizeAgent(agent) == "dsh" {
		out := map[string]string{}
		if rows, err := dsh.Sessions.List(); err == nil {
			for _, s := range rows {
				out[s.ID] = strings.Join(strings.Fields(s.Title), " ")
			}
		}
		return out
	}
	if normalizeAgent(agent) == "agy" {
		return agySessionTitles()
	}
	if normalizeAgent(agent) != "codex" {
		return nil
	}
	indexPath := filepath.Join(codexHome(), "session_index.jsonl")
	f, err := os.Open(indexPath)
	if err != nil {
		return nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	ino := fileIno(info)
	size := info.Size()
	mtimeNano := info.ModTime().UnixNano()
	codexTitlesMu.Lock()
	if codexTitlesState.titles != nil && codexTitlesState.path == indexPath &&
		codexTitlesState.ino == ino && codexTitlesState.size == size && codexTitlesState.mtimeNano == mtimeNano {
		cloned := maps.Clone(codexTitlesState.titles)
		codexTitlesMu.Unlock()
		return cloned
	}
	codexTitlesMu.Unlock()
	// The index is append-only: its recent tail contains new sessions and
	// renames. Bound polling cost even when years of sessions have accumulated.
	start := max(int64(0), size-titleIndexLimit)
	buf := make([]byte, size-start)
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
	codexTitlesMu.Lock()
	codexTitlesState.path = indexPath
	codexTitlesState.ino = ino
	codexTitlesState.size = size
	codexTitlesState.mtimeNano = mtimeNano
	codexTitlesState.titles = maps.Clone(titles)
	codexTitlesMu.Unlock()
	return titles
}
