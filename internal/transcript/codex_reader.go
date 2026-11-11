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
						return subtype == "message" || subtype == "function_call" || subtype == "custom_tool_call"
					}
					if subtype == "item_completed" {
						return codexNativeHeader(d)
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
		if st.stop {
			return next, nil
		}
	}
}

// Codex rollouts can contain gigabytes of tool/compaction records inside a
// single user turn. Start from the beginning once, retain chat turns, then read
// only appended records (not the whole active turn) on subsequent polls.
func (l *Loader) loadCodexFile(path string, maxTurns int) ([]Turn, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	hit := l.cached(path)
	f, err := os.Open(path)
	if err != nil {
		delete(l.m, path)
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	limit := maxTurns
	if limit <= 0 || limit > cacheTurnCap {
		limit = cacheTurnCap
	}
	st := &parseState{turnCap: limit, sourceID: recordSourceID(path)}
	start := int64(0)
	if hit != nil && codexCacheCovers(hit.turns, limit) && hit.file != nil && os.SameFile(hit.file, fi) {
		if fi.Size() == hit.size && fi.ModTime().Equal(hit.mtime) {
			return lastN(hit.turns, maxTurns), nil
		}
		if fi.Size() > hit.size {
			start = hit.nextLine
			st.pendingContexts = append([]ContextRecord(nil), hit.pendingContexts...)
			st.pendingContextStart = hit.pendingContextStart
			if n := len(hit.turns); n > 0 {
				st.turns = append([]Turn(nil), hit.turns[:n-1]...)
				st.turnStarts = make([]int64, n-1)
				// Previous snapshots may still be in use by HTTP handlers. Clone the active
				// turn and nested slices before appending any text or tools to it.
				cur := hit.turns[n-1]
				cur.Segments = append([]Segment(nil), cur.Segments...)
				for i := range cur.Segments {
					cur.Segments[i].Steps = append([]Step(nil), cur.Segments[i].Steps...)
					for _, step := range cur.Segments[i].Steps {
						st.toolDetailBytes += len(step.Input) + len(step.Workdir)
					}
				}
				st.cur, st.curStart = &cur, hit.lastTurn
				st.nextOrdinal = cur.Ordinal - 1
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
	l.retainCodex(&cacheEntry{path: path, size: fi.Size(), turns: st.turns, lastTurn: last, nextLine: next, file: fi, mtime: fi.ModTime(), pendingContexts: st.pendingContexts, pendingContextStart: st.pendingContextStart})
	return lastN(st.turns, maxTurns), nil
}

// A bounded snapshot is reusable only for requests it completely covers. Larger
// requests re-read the authoritative source, including when that source appended.
func codexCacheCovers(turns []Turn, limit int) bool {
	return len(turns) == 0 || turns[0].Ordinal == 1 || len(turns) >= limit
}

// Keep a whole-turn tail without changing the caller's requested result. In
// particular, an oversized last turn is returned intact even when it cannot fit.
func (l *Loader) retainCodex(e *cacheEntry) {
	metadata := int64(2*len(e.path) + 256)
	for _, record := range e.pendingContexts {
		metadata += int64(128 + len(record.Text) + len(record.Source) + len(record.Title) + len(record.Time))
	}
	total := cachedTurnBytes(e.turns) + metadata
	first := 0
	for total > cacheByteCap && first < len(e.turns)-1 {
		total -= cachedTurnBytes(e.turns[first : first+1])
		first++
	}
	// A new slice releases the omitted prefix; it also keeps cache retention from
	// modifying slices already returned to an HTTP reader.
	e.turns = append([]Turn(nil), e.turns[first:]...)
	l.retain(e)
}
