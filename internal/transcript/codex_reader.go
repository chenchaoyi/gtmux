package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
)

// codexChatRecord classifies only structural headers, never text embedded in a
// tool result. Incomplete or unfamiliar headers fall back to the normal parser.
// This lets large screenshots, tool results and compaction snapshots stream by
// without allocating their entire JSON record or spending a chat byte budget.
func codexChatRecord(prefix []byte) bool {
	d := json.NewDecoder(bytes.NewReader(prefix))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return true
	}
	kind := ""
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return true
		}
		if key == "type" {
			if d.Decode(&kind) != nil {
				return true
			}
			if kind != "response_item" && kind != "event_msg" {
				return false
			}
		} else if key == "payload" {
			if kind == "" {
				return true
			}
			token, err := d.Token()
			if err != nil || token != json.Delim('{') {
				return true
			}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return true
				}
				if key == "type" {
					var subtype string
					if d.Decode(&subtype) != nil {
						return true
					}
					if kind == "response_item" {
						return subtype == "message" || subtype == "function_call"
					}
					return subtype == "user_message" || subtype == "agent_message" || subtype == "task_complete"
				}
				var discard json.RawMessage
				if d.Decode(&discard) != nil {
					return true
				}
			}
			return true
		} else {
			var discard json.RawMessage
			if d.Decode(&discard) != nil {
				return true
			}
		}
	}
	return true
}

// readCodexRecords consumes complete records only. A partially written final
// record is retried from its start on the next poll, without a phantom turn.
func readCodexRecords(r io.Reader, start int64, st *parseState) (int64, error) {
	reader := bufio.NewReaderSize(r, 64*1024)
	next := start
	for {
		part, err := reader.ReadSlice('\n')
		if len(part) == 0 && err == io.EOF {
			return next, nil
		}
		keep := codexChatRecord(part)
		length := int64(len(part))
		var record []byte
		if keep {
			record = append(record, part...)
		}
		for err == bufio.ErrBufferFull {
			part, err = reader.ReadSlice('\n')
			length += int64(len(part))
			if keep {
				record = append(record, part...)
			}
		}
		if err == io.EOF {
			return next, nil
		}
		if err != nil {
			return next, err
		}
		st.off = next
		if keep {
			codexStep(string(record), st)
		}
		next += length
	}
}

// Codex rollouts can contain gigabytes of tool/compaction records inside a
// single user turn. Start from the beginning once, retain chat turns, then read
// only appended records (not the whole active turn) on subsequent polls.
func (l *Loader) loadCodexFile(path string, maxTurns int) ([]Turn, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	st := &parseState{turnCap: cacheTurnCap}
	start := int64(0)
	if hit := l.m[path]; hit != nil && hit.file != nil && os.SameFile(hit.file, fi) {
		if fi.Size() == hit.size && fi.ModTime().Equal(hit.mtime) {
			return lastN(hit.turns, maxTurns), nil
		}
		if fi.Size() > hit.size {
			start = hit.nextLine
			if n := len(hit.turns); n > 0 {
				st.turns = append([]Turn(nil), hit.turns[:n-1]...)
				st.turnStarts = make([]int64, n-1)
				// Previous snapshots may still be in use by HTTP handlers. Clone the active
				// turn and nested slices before appending any text or tools to it.
				cur := hit.turns[n-1]
				cur.Segments = append([]Segment(nil), cur.Segments...)
				for i := range cur.Segments {
					cur.Segments[i].Steps = append([]Step(nil), cur.Segments[i].Steps...)
				}
				st.cur, st.curStart = &cur, hit.lastTurn
			}
		}
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	next, err := readCodexRecords(io.LimitReader(f, fi.Size()-start), start, st)
	if err != nil {
		return nil, err
	}
	st.flush()
	last := int64(-1)
	if n := len(st.turnStarts); n > 0 {
		last = st.turnStarts[n-1]
	}
	if l.m == nil {
		l.m = make(map[string]*cacheEntry)
	}
	l.m[path] = &cacheEntry{path: path, size: fi.Size(), turns: st.turns, lastTurn: last, nextLine: next, file: fi, mtime: fi.ModTime()}
	return lastN(st.turns, maxTurns), nil
}
