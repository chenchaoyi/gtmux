package events

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"sync"
)

// SendIndex is the journal's deliveries, kept current for a reader that lives long.
//
// The chat attributes each prompt to whoever delivered it (who-sent-this-turn), and it
// answered that by parsing the whole journal: every retained record, both generations, on
// every chat request. Measured on a real 16 MB journal it cost about 110ms whatever the
// window (a full scan, then a filter), and an open chat asks every 8 seconds, almost
// always to be told nothing changed (found 2026-09-22).
//
// Almost none of the journal is deliveries, and the journal only grows at its end. So
// this holds the deliveries alone, and on each call it reads only the bytes appended
// since the last one. It starts over when the active file is not the file it was reading
// (rotation renamed it) or is shorter than what was read.
type SendIndex struct {
	mu     sync.Mutex
	file   os.FileInfo // the active generation last read, for identity
	offset int64       // bytes of it already folded in (always at a line end)
	byPane map[string]map[string]sendEntry
	// parsed counts the records decoded, so a test can see a call read only the tail.
	parsed int
}

type sendEntry struct {
	actor string
	ts    int64
}

// Sends is the index a long-lived process shares.
var Sends = &SendIndex{}

// Senders returns, for one pane, who delivered each head recorded at or after cutoff
// (unix seconds; 0 = all retained). The newest delivery of a head wins, as it did when
// the map was built from the records in order.
func (x *SendIndex) Senders(pane string, cutoff int64) map[string]string {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.refreshLocked()
	out := map[string]string{}
	for head, e := range x.byPane[pane] {
		if e.ts >= cutoff {
			out[head] = e.actor
		}
	}
	return out
}

func (x *SendIndex) refreshLocked() {
	fi, err := os.Stat(Path())
	if err != nil {
		x.file, x.offset, x.byPane = nil, 0, nil
		return
	}
	if x.file == nil || !os.SameFile(x.file, fi) || fi.Size() < x.offset || x.byPane == nil {
		// Rotated, replaced or truncated: what was folded in no longer describes the file.
		x.byPane = map[string]map[string]sendEntry{}
		x.offset = 0
		x.foldFile(rotatedPath(), 0) // the older generation first, so newer wins
	}
	x.file = fi
	if fi.Size() > x.offset {
		x.offset = x.foldFile(Path(), x.offset)
	}
}

// foldFile folds the complete lines of path from offset on, and returns the offset just
// past the last one. A line still being written has no newline yet, so it is left for
// the next call rather than decoded half-written and lost.
func (x *SendIndex) foldFile(path string, offset int64) int64 {
	f, err := os.Open(path)
	if err != nil {
		return offset
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset
	}
	r := bufio.NewReaderSize(f, 64*1024)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return offset // EOF, possibly mid-line: stop before the partial line
		}
		offset += int64(len(line))
		if !bytes.Contains(line, []byte(AuditEventSend)) {
			continue // not a delivery; skip the decode entirely
		}
		var rec Record
		if json.Unmarshal(line, &rec) != nil {
			continue
		}
		x.parsed++
		if pane, head, actor, ok := landedSend(rec); ok {
			if x.byPane[pane] == nil {
				x.byPane[pane] = map[string]sendEntry{}
			}
			x.byPane[pane][head] = sendEntry{actor: actor, ts: rec.Ts}
		}
	}
}
