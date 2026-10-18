package events

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
)

// HQSessionIndex retains only the HQ handoff links. Like SendIndex, it folds
// appended complete lines and rebuilds on rotation/replacement/truncation, so a
// chat poll never materializes the full lifecycle journal.
type HQSessionIndex struct {
	mu     sync.Mutex
	file   os.FileInfo
	offset int64
	links  map[string]hqSessionLink
	parsed int // relevant records decoded, for incremental-read regression tests
}

type hqSessionLink struct{ session, agent string }

var hqSessions = &HQSessionIndex{}

func (x *HQSessionIndex) Predecessor(successor string) (string, string) {
	if successor == "" {
		return "", ""
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	fi, err := os.Stat(Path())
	if err != nil {
		x.file, x.offset = nil, 0
		x.links = map[string]hqSessionLink{}
		if os.IsNotExist(err) {
			x.foldFile(rotatedPath(), 0)
		}
		link := x.links[successor]
		return link.session, link.agent
	}
	if x.file == nil || !os.SameFile(x.file, fi) || fi.Size() < x.offset || x.links == nil {
		x.links = map[string]hqSessionLink{}
		x.offset = 0
		x.foldFile(rotatedPath(), 0)
	}
	x.file = fi
	if fi.Size() > x.offset {
		x.offset = x.foldFile(Path(), x.offset)
	}
	link := x.links[successor]
	return link.session, link.agent
}

func (x *HQSessionIndex) foldFile(path string, offset int64) int64 {
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
			return offset
		} // leave a partial append for the next read
		offset += int64(len(line))
		if !bytes.Contains(line, []byte(AuditEventHQSession)) {
			continue
		}
		var rec Record
		if json.Unmarshal(line, &rec) != nil || rec.Event != AuditEventHQSession {
			continue
		}
		x.parsed++
		legacy, previous, ok := strings.Cut(rec.Summary, " replaces ")
		link := hqSessionLink{rec.PreviousAgentSession, rec.PreviousAgentKey}
		if rec.AgentSession != "" {
			structured := link
			if structured.session == "" && ok && legacy == rec.AgentSession {
				structured = hqSessionLink{session: previous}
			}
			x.links[rec.AgentSession] = structured
		}
		if ok && legacy != "" {
			if link.session == "" {
				link = hqSessionLink{session: previous}
			}
			x.links[legacy] = link // newest record wins, including legacy summaries
		}
	}
}
