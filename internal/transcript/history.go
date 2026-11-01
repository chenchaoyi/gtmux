package transcript

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/chenchaoyi/gtmux/internal/dsh"
)

// History cursors are read positions inside an already authorized session, never
// filesystem paths or grants. Appends preserve them; replacement/rewrite of the
// anchor requires a fresh snapshot instead of silently joining unrelated history.
var ErrHistoryChanged = errors.New("transcript history changed")

const HistoryPageTurns = 50

type historyCursor struct {
	Session    string `json:"s"`
	Record     string `json:"r"`
	Ordinal    int    `json:"n"`
	Anchor     string `json:"a"`
	Generation string `json:"g"`
	Size       int64  `json:"z"`
	Modified   int64  `json:"m"`
}

func historyPaths(agent, session string) []string {
	if normalizeAgent(agent) == "codex" {
		return codexLogPaths(session)
	}
	if path, _ := resolveLog(agent, session); path != "" {
		return []string{path}
	}
	return nil
}
func historyAnchor(t Turn) string {
	h := sha256.Sum256([]byte(t.RecordID + "\x00" + t.Time + "\x00" + t.Prompt))
	return fmt.Sprintf("%x", h[:12])
}
func historyGeneration(path string) string {
	f, e := os.Open(path)
	if e != nil {
		return ""
	}
	defer f.Close()
	fi, e := f.Stat()
	if e != nil {
		return ""
	}
	b := make([]byte, 64)
	n, _ := f.ReadAt(b, 0)
	h := sha256.Sum256(b[:n])
	return fmt.Sprintf("%x-%x", fileIno(fi), h[:8])
}
func historyPathFor(agent, session, record string) string {
	for _, p := range historyPaths(agent, session) {
		if strings.HasPrefix(record, recordSourceID(p)+":") || normalizeAgent(agent) == "dsh" {
			return p
		}
	}
	return ""
}

// SetHistoryCursor annotates only the oldest returned turn. The next request is
// bounded independently of how many earlier pages the reader already holds.
func SetHistoryCursor(agent, session string, turns []Turn) {
	if len(turns) == 0 || turns[0].Ordinal <= 1 {
		return
	}
	t := &turns[0]
	path := historyPathFor(agent, session, t.RecordID)
	if path == "" {
		return
	}
	g := historyGeneration(path)
	if g == "" {
		return
	}
	fi, e := os.Stat(path)
	if e != nil {
		return
	}
	b, _ := json.Marshal(historyCursor{Session: session, Record: t.RecordID, Ordinal: t.Ordinal, Anchor: historyAnchor(*t), Generation: g, Size: fi.Size(), Modified: fi.ModTime().UnixNano()})
	t.HistoryBefore = base64.RawURLEncoding.EncodeToString(b)
}
func HistorySession(cursor string) string {
	c, e := decodeHistoryCursor(cursor)
	if e != nil {
		return ""
	}
	return c.Session
}
func decodeHistoryCursor(s string) (historyCursor, error) {
	var c historyCursor
	if len(s) > 2048 {
		return c, ErrHistoryChanged
	}
	b, e := base64.RawURLEncoding.DecodeString(s)
	if e != nil || json.Unmarshal(b, &c) != nil || c.Session == "" || c.Record == "" || c.Ordinal < 2 || c.Anchor == "" || c.Generation == "" {
		return c, ErrHistoryChanged
	}
	return c, nil
}

// LoadBefore streams canonical turn boundaries up to the anchor, retaining at
// most one page. This explicit history read bypasses the newest-turn cache so
// older records remain reachable after cache eviction and its 800-turn limit.
func LoadBefore(ctx context.Context, agent, session, before string) ([]Turn, error) {
	c, e := decodeHistoryCursor(before)
	if e != nil || c.Session != session {
		return nil, ErrHistoryChanged
	}
	anchorPath := historyPathFor(agent, session, c.Record)
	if anchorPath == "" || historyGeneration(anchorPath) != c.Generation {
		return nil, ErrHistoryChanged
	}
	fi, e := os.Stat(anchorPath)
	if e != nil || fi.Size() < c.Size || fi.Size() == c.Size && fi.ModTime().UnixNano() != c.Modified {
		return nil, ErrHistoryChanged
	}
	paths := historyPaths(agent, session)
	var turns []Turn
	found := false
	ordinal := 0
	start := int64(0)
	var boundaries []historyBoundary
	indexed := false
	if len(paths) == 1 && normalizeAgent(agent) != "dsh" {
		if entry, ok := getHistoryIndex(anchorPath, fi); ok {
			target := max(1, c.Ordinal-HistoryPageTurns)
			for _, b := range entry.bounds {
				if b.ordinal == target {
					start = b.offset
					ordinal = target - 1
					indexed = true
					break
				}
			}
		}
	}
	visit := func(t Turn) bool {
		ordinal++
		t.Ordinal = ordinal
		if len(paths) == 1 && !indexed && normalizeAgent(agent) != "dsh" {
			if off, err := strconv.ParseInt(strings.TrimPrefix(t.RecordID, recordSourceID(anchorPath)+":"), 10, 64); err == nil {
				boundaries = append(boundaries, historyBoundary{ordinal, off})
				if len(boundaries) > historyIndexTurns {
					boundaries = append([]historyBoundary(nil), boundaries[len(boundaries)-historyIndexTurns:]...)
				}
			}
		}
		if t.RecordID == c.Record {
			found = t.Ordinal == c.Ordinal && historyAnchor(t) == c.Anchor
			return false
		}
		turns = append(turns, t)
		for len(turns) > 1 && (len(turns) > HistoryPageTurns || cachedTurnBytes(turns) > 2<<20) {
			turns = append([]Turn(nil), turns[1:]...)
		}
		return true
	}
	for _, path := range paths {
		if e = scanHistory(ctx, path, agent, session, start, visit); e != nil {
			return nil, e
		}
		if found {
			break
		}
		if path == anchorPath {
			return nil, ErrHistoryChanged
		}
	}
	if !found || historyGeneration(anchorPath) != c.Generation {
		return nil, ErrHistoryChanged
	}
	if !indexed && len(boundaries) > 0 {
		putHistoryIndex(anchorPath, historyIndex{fi, boundaries})
	}
	return turns, nil
}

type historyReader struct {
	ctx context.Context
	r   io.Reader
}

func (r historyReader) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(p)
}
func scanHistory(ctx context.Context, path, agent, session string, start int64, visit func(Turn) bool) error {
	st := &parseState{sourceID: recordSourceID(path), turnCap: 1, onTurn: visit}
	if normalizeAgent(agent) == "dsh" {
		cursor := dsh.Cursor{}
		header := false
		e := dsh.Scan(path, &cursor, func() {}, func(line []byte) error {
			if e := ctx.Err(); e != nil {
				return e
			}
			if !header {
				var h dsh.Header
				if json.Unmarshal(line, &h) != nil || h.ID != session || h.Version != 4 {
					return ErrHistoryChanged
				}
				header = true
				return nil
			}
			if e := dshStep(line, st); e != nil {
				return e
			}
			if st.stop {
				return io.EOF
			}
			return nil
		})
		if e != nil && e != io.EOF {
			return e
		}
		if !st.stop {
			st.flush()
		}
		return nil
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	fi, e := f.Stat()
	if e != nil {
		return e
	}
	if _, e = f.Seek(start, io.SeekStart); e != nil {
		return e
	}
	r := historyReader{ctx, io.LimitReader(f, fi.Size()-start)}
	if normalizeAgent(agent) == "codex" {
		_, e = readCodexRecords(r, start, st)
	} else {
		_, step := resolveLog(agent, session)
		if step == nil {
			return ErrHistoryChanged
		}
		reader := bufio.NewReaderSize(r, 64<<10)
		off := start
		for !st.stop {
			line, err := reader.ReadString('\n')
			if len(line) > 0 && err == nil {
				st.off = off
				step(strings.TrimRight(line, "\n"), st)
				off += int64(len(line))
			}
			if err != nil {
				if err != io.EOF {
					e = err
				}
				break
			}
		}
	}
	if e != nil {
		return e
	}
	if !st.stop {
		st.flush()
	}
	return ctx.Err()
}

// Only reusable byte positions are cached: no messages, paths in cursors, or
// persistent copies. At most sixteen files and 8,192 boundaries per file.
const historyIndexTurns = 8192

type historyBoundary struct {
	ordinal int
	offset  int64
}
type historyIndex struct {
	file   os.FileInfo
	bounds []historyBoundary
}

var historyIndices = struct {
	sync.Mutex
	entries map[string]historyIndex
}{entries: map[string]historyIndex{}}

func getHistoryIndex(path string, fi os.FileInfo) (historyIndex, bool) {
	historyIndices.Lock()
	defer historyIndices.Unlock()
	entry, ok := historyIndices.entries[path]
	if !ok {
		return entry, false
	}
	if !os.SameFile(entry.file, fi) || fi.Size() < entry.file.Size() || fi.Size() == entry.file.Size() && !fi.ModTime().Equal(entry.file.ModTime()) {
		delete(historyIndices.entries, path)
		return historyIndex{}, false
	}
	return entry, true
}
func putHistoryIndex(path string, entry historyIndex) {
	historyIndices.Lock()
	defer historyIndices.Unlock()
	if len(historyIndices.entries) >= 16 {
		for key := range historyIndices.entries {
			delete(historyIndices.entries, key)
			break
		}
	}
	historyIndices.entries[path] = entry
}
